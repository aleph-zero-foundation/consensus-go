package tcoin

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/validator-skeleton/pkg/crypto/bn256"
)

// NewGlobal returns a GlobalThresholdCoin based on given slice of coefficients.
func NewGlobal(nProc uint16, coeffs []*big.Int) *GlobalThresholdCoin {
	threshold := uint16(len(coeffs))
	secret := coeffs[threshold-1]

	globalVK := bn256.NewVerificationKey(secret)

	var wg sync.WaitGroup
	var sks = make([]*bn256.SecretKey, nProc)
	var vks = make([]*bn256.VerificationKey, nProc)

	for i := uint16(0); i < nProc; i++ {
		wg.Add(1)
		go func(ind uint16) {
			defer wg.Done()
			secret := poly(coeffs, big.NewInt(int64(ind+1)))
			sks[ind] = bn256.NewSecretKey(secret)
			vks[ind] = bn256.NewVerificationKey(secret)
		}(i)
	}
	wg.Wait()

	return &GlobalThresholdCoin{
		threshold: threshold,
		globalVK:  globalVK,
		vks:       vks,
		sks:       sks,
	}
}

// NewRandomGlobal generates a random polynomial of degree thereshold - 1 and builds
// a GlobalThresholdCoin based on the polynomial.
func NewRandomGlobal(nProc, threshold uint16) *GlobalThresholdCoin {
	var coeffs = make([]*big.Int, threshold)
	for i := uint16(0); i < threshold; i++ {
		c, _ := rand.Int(rand.Reader, bn256.Order)
		coeffs[i] = c
	}
	return NewGlobal(nProc, coeffs)
}

// Encrypt encrypts secretKeys of the given GlobalThresholdCoin
// using given set of encryptionKeys and returns a (unowned)ThresholdCoin.
func (gtc *GlobalThresholdCoin) Encrypt(encryptionKeys []encrypt.SymmetricKey) (*ThresholdCoin, error) {
	nProc := uint16(len(encryptionKeys))
	encSKs := make([]encrypt.CipherText, nProc)

	for i := uint16(0); i < nProc; i++ {
		encSK, err := encryptionKeys[i].Encrypt(gtc.sks[i].Marshal())
		if err != nil {
			return nil, err
		}
		encSKs[i] = encSK
	}

	return &ThresholdCoin{
		threshold: gtc.threshold,
		globalVK:  gtc.globalVK,
		vks:       gtc.vks,
		encSKs:    encSKs,
	}, nil
}

// Encode returns a byte representation of the given (unowned) ThresholdCoin.
// in the following form
// (1) threshold, 2 bytes as uint16
// (2) length of marshalled globalVK, 4 bytes as uint32
// (3) marshalled globalVK
// (4) len(vks), 4 bytes as uint32
// (5) Marshalled vks in the form
//     a) length of marshalled vk
//     b) marshalled vk
// (6) Encrypted sks in the form
//     a) length of the cipher text
//     b) cipher text of the key
func (tc *ThresholdCoin) Encode() []byte {
	data := make([]byte, 2+4)
	binary.LittleEndian.PutUint16(data[:2], tc.threshold)

	globalVKMarshalled := tc.globalVK.Marshal()
	binary.LittleEndian.PutUint32(data[2:6], uint32(len(globalVKMarshalled)))
	data = append(data, globalVKMarshalled...)

	dataLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(dataLen[:], uint32(len(tc.vks)))
	data = append(data, dataLen...)

	for _, vk := range tc.vks {
		vkMarshalled := vk.Marshal()
		binary.LittleEndian.PutUint32(dataLen, uint32(len(vkMarshalled)))
		data = append(data, dataLen...)
		data = append(data, vkMarshalled...)
	}

	for _, encSK := range tc.encSKs {
		binary.LittleEndian.PutUint32(dataLen, uint32(len(encSK)))
		data = append(data, dataLen...)
		data = append(data, encSK...)
	}
	return data
}

// Decode decodes encoded ThresholdCoin obtained from the dealer using given decryptionKey.
// It returns
// (1) decoded ThresholdCoin,
// (2) whether the owner's secretKey is correctly encoded and matches corresponding verification key,
// (3) an error in decoding (excluding errors obtained while decoding owners secret key),
func Decode(data []byte, dealer, owner uint16, decryptionKey encrypt.SymmetricKey) (*ThresholdCoin, bool, error) {
	ind := 0
	dataTooShort := errors.New("Decoding tcoin failed. Given bytes slice is too short")
	if len(data) < ind+2 {
		return nil, false, dataTooShort
	}
	threshold := binary.LittleEndian.Uint16(data[:(ind + 2)])
	ind += 2

	if len(data) < ind+4 {
		return nil, false, dataTooShort
	}
	gvkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	if len(data) < ind+gvkLen {
		return nil, false, dataTooShort
	}
	globalVK, err := new(bn256.VerificationKey).Unmarshal(data[ind:(ind + gvkLen)])
	if err != nil {
		return nil, false, errors.New("unmarshal of globalVK failed")
	}
	ind += gvkLen

	if len(data) < ind+4 {
		return nil, false, dataTooShort
	}
	nProcesses := uint16(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	vks := make([]*bn256.VerificationKey, nProcesses)
	for i := range vks {
		if len(data) < ind+4 {
			return nil, false, dataTooShort
		}
		vkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		if len(data) < ind+vkLen {
			return nil, false, dataTooShort
		}
		vks[i], err = new(bn256.VerificationKey).Unmarshal(data[ind:(ind + vkLen)])
		if err != nil {
			return nil, false, errors.New("unmarshal of vk failed")
		}
		ind += vkLen
	}
	encSKs := make([]encrypt.CipherText, nProcesses)
	for i := range encSKs {
		if len(data) < ind+4 {
			return nil, false, dataTooShort
		}
		skLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		if len(data) < ind+skLen {
			return nil, false, dataTooShort
		}
		encSKs[i] = data[ind:(ind + skLen)]
		ind += skLen
	}

	sk, err := decryptSecretKey(encSKs[owner], vks[owner], decryptionKey)

	return &ThresholdCoin{
		dealer:    dealer,
		owner:     owner,
		threshold: threshold,
		globalVK:  globalVK,
		vks:       vks,
		encSKs:    encSKs,
		sk:        sk,
	}, (err == nil), nil
}

func decryptSecretKey(data []byte, vk *bn256.VerificationKey, decryptionKey encrypt.SymmetricKey) (*bn256.SecretKey, error) {
	decrypted, err := decryptionKey.Decrypt(data)
	if err != nil {
		return nil, err
	}

	sk, err := new(bn256.SecretKey).Unmarshal(decrypted)
	if err != nil {
		return nil, err
	}

	if !bn256.VerifyKeys(vk, sk) {
		return nil, errors.New("secret key doesn't match with the verification key")
	}
	return sk, nil
}

// CheckSecretKey checks whether the secret key of the given pid is correct.
func (tc *ThresholdCoin) CheckSecretKey(pid uint16, decryptionKey encrypt.SymmetricKey) bool {
	_, err := decryptSecretKey(tc.encSKs[pid], tc.vks[pid], decryptionKey)
	return err == nil
}
