package creator

import (
	"encoding/binary"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

// proof is a message required to verify if the epoch has finished.
// It consists of id and hash of the last timing unit of the epoch.
// This message is signed with a threshold signature.
const proofLength = gomel.HashLength + 8

// EpochProof checks if the given preunit is a proof that a new epoch started.
func EpochProof(pu gomel.Preunit, wtk *tss.WeakThresholdKey) bool {
	if !gomel.Dealing(pu) || wtk == nil {
		return false
	}
	if pu.EpochID() == gomel.EpochID(0) {
		return true
	}
	sig, msg, err := decodeSignature(pu.Data())
	if err != nil {
		return false
	}
	_, _, epoch, _ := decodeProof(msg)
	if epoch+1 != pu.EpochID() {
		return false
	}
	return wtk.VerifySignature(sig, msg)
}

// encodeProof produces encoded form of the proof that the current epoch ended.
// The proof consists of id and hash of the last timing unit of the epoch.
func encodeProof(u gomel.Unit) []byte {
	msg := make([]byte, proofLength)
	binary.LittleEndian.PutUint64(msg[:8], gomel.UnitID(u))
	copy(msg[8:], u.Hash()[:])
	return msg
}

// decodeProof takes encoded proof and decodes it into height, creator, epoch and hash of last timing unit of the epoch.
func decodeProof(msg []byte) (int, uint16, gomel.EpochID, *gomel.Hash) {
	if len(msg) == proofLength {
		id := binary.LittleEndian.Uint64(msg[:8])
		var hash gomel.Hash
		copy(hash[:], msg[8:])
		h, c, e := gomel.DecodeID(id)
		return h, c, e, &hash
	}
	return -1, 0, 0, nil
}

// encodeShare converts signature share and the signed message into Data that can be put into unit.
func encodeShare(share *tss.Share, msg []byte) core.Data {
	result := append([]byte{}, msg...)
	result = append(result, share.Marshal()...)
	return core.Data(result)
}

// decodeShare reads signature share and the signed message from Data contained in some unit.
func decodeShare(data core.Data) (*tss.Share, []byte, error) {
	result := new(tss.Share)
	err := result.Unmarshal(data[proofLength:])
	if err != nil {
		return nil, nil, err
	}
	return result, data[:proofLength], nil
}

// encodeSignature converts signature and the signed message into Data that can be put into unit.
func encodeSignature(sig *tss.Signature, msg []byte) core.Data {
	result := append([]byte{}, msg...)
	result = append(result, sig.Marshal()...)
	return core.Data(result)
}

// decodeSignature reads signature and the signed message from Data contained in some unit.
func decodeSignature(data core.Data) (*tss.Signature, []byte, error) {
	result := new(tss.Signature)
	err := result.Unmarshal(data[proofLength:])
	if err != nil {
		return nil, nil, err
	}
	return result, data[:proofLength], nil
}

// shareDB is a simple storage for threshold signature shares indexed by the message they sign.
type shareDB struct {
	conf config.Config
	data map[string]map[uint16]*tss.Share
}

// newShareDB constructs a storage for shares that uses the provided weak threshold key for combining shares.
func newShareDB(conf config.Config) *shareDB {
	return &shareDB{conf: conf, data: make(map[string]map[uint16]*tss.Share)}
}

// Add puts the share that signs msg to the storage. If there are enough shares (for that msg),
// they are combined and the resulting signature is returned. Otherwise, returns nil.
func (db *shareDB) Add(share *tss.Share, msg []byte) *tss.Signature {
	key := string(msg)
	shares, ok := db.data[key]
	if !ok {
		shares = make(map[uint16]*tss.Share)
		db.data[key] = shares
	}
	shares[share.Owner()] = share
	if len(shares) >= int(db.conf.WTKey.Threshold()) {
		shareSlice := make([]*tss.Share, 0, len(shares))
		for _, share := range shares {
			shareSlice = append(shareSlice, share)
		}
		if sig, ok := db.conf.WTKey.CombineShares(shareSlice); ok {
			return sig
		}
	}
	return nil
}

// EpochProofBuilder is a type responsible for building and verifying so called epoch-proofs.
type EpochProofBuilder interface {
	// Verify checks if given unit is a valid proof of epoch pu.EpochID()-1.
	Verify(gomel.Preunit) bool
	// TryBuilding attempts to construct an epoch-proof.
	TryBuilding(gomel.Unit) core.Data
	// BuildShare creates our share of the epoch-proof.
	BuildShare(lastTimingUnit gomel.Unit) core.Data
}

type epochProofImpl struct {
	conf   config.Config
	epoch  gomel.EpochID
	shares *shareDB
	log    zerolog.Logger
}

// NewProofBuilder creates an instance of the EpochProofBuilder type.
func NewProofBuilder(conf config.Config, log zerolog.Logger) func(gomel.EpochID) EpochProofBuilder {
	return func(epoch gomel.EpochID) EpochProofBuilder {
		return &epochProofImpl{
			conf:   conf,
			epoch:  epoch,
			shares: newShareDB(conf),
			log:    log,
		}
	}
}

func (epi *epochProofImpl) BuildShare(lastTimingUnit gomel.Unit) core.Data {
	msg := encodeProof(lastTimingUnit)
	share := epi.conf.WTKey.CreateShare(msg)
	if share != nil {
		return encodeShare(share, msg)
	}
	return core.Data{}
}

func (epi *epochProofImpl) Verify(pu gomel.Preunit) bool {
	if epi.epoch+1 != pu.EpochID() {
		return false
	}
	return EpochProof(pu, epi.conf.WTKey)
}

// updateShares extracts threshold signature shares from finishing units.
// If there are enough shares to combine, it produces the signature and
// converts it to core.Data. Otherwise, nil is returned.
func (epi *epochProofImpl) TryBuilding(u gomel.Unit) core.Data {
	// ignore regular units and finishing units with empty data
	if u.Level() < epi.conf.OrderStartLevel+epi.conf.EpochLength || len(u.Data()) == 0 {
		return nil
	}
	share, msg, err := decodeShare(u.Data())
	if err != nil {
		epi.log.Error().Str("where", "creator.decodeShare").Msg(err.Error())
		return nil
	}
	if !epi.conf.WTKey.VerifyShare(share, msg) {
		epi.log.Error().Str("where", "creator.verifyShare").Msg(err.Error())
		return nil
	}
	sig := epi.shares.Add(share, msg)
	if sig != nil {
		return encodeSignature(sig, msg)
	}
	return nil
}
