package creator

import (
	"encoding/binary"

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
	// TODO consider adding a proof that beacon has finished
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
	return core.Data(append(msg, share.Marshal()...))
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
	return core.Data(append(msg, sig.Marshal()...))
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
	data map[string][]*tss.Share
}

// newShareDB constructs a storage for shares that uses the provided weak threshold key for combining shares.
func newShareDB(conf config.Config) *shareDB {
	return &shareDB{conf: conf, data: make(map[string][]*tss.Share)}
}

// add puts the share that signs msg to the storage. If there are enough shares (for that msg),
// they are combined and the resulting signature is returned. Otherwise, returns nil.
func (db *shareDB) add(share *tss.Share, msg []byte) *tss.Signature {
	key := string(msg)
	if shares, ok := db.data[key]; ok {
		db.data[key] = append(shares, share)
	} else {
		db.data[key] = []*tss.Share{share}
	}
	if len(db.data[key]) >= int(db.conf.WTKey.Threshold()) {
		if sig, ok := db.conf.WTKey.CombineShares(db.data[key]); ok {
			return sig
		}
	}
	return nil
}

// reset empties the storage and brings it back to the initial state.
func (db *shareDB) reset() {
	db.data = make(map[string][]*tss.Share)
}
