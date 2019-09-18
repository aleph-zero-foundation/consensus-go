package tcoin

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
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
func (gtc *GlobalThresholdCoin) Encrypt(dealer uint16, encryptionKeys []encrypt.EncryptionKey) (*ThresholdCoin, error) {
	nProc := uint16(len(encryptionKeys))
	encSKs := make([]encrypt.CipherText, nProc)

	// We encrypt (dealer, secretKey) to avoid the copying attack.
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, dealer)
	for i := uint16(0); i < nProc; i++ {
		skMarshalled := gtc.sks[i].Marshal()
		encSK, err := encryptionKeys[i].Encrypt(append(buf, skMarshalled...))
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

// Decode decodes encoded ThresholdCoin obtained from the sender using given decryptionKey.
func Decode(data []byte, sender, owner uint16, decryptionKey encrypt.DecryptionKey) (*ThresholdCoin, error) {
	ind := 0
	dataTooShort := errors.New("Decoding tcoin failed. Given bytes slice is too short")
	if len(data) < ind+2 {
		return nil, dataTooShort
	}
	threshold := binary.LittleEndian.Uint16(data[:(ind + 2)])
	ind += 2

	if len(data) < ind+4 {
		return nil, dataTooShort
	}
	gvkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	if len(data) < ind+gvkLen {
		return nil, dataTooShort
	}
	globalVK, err := new(bn256.VerificationKey).Unmarshal(data[ind:(ind + gvkLen)])
	if err != nil {
		return nil, errors.New("unmarshal of globalVK failed")
	}
	ind += gvkLen

	if len(data) < ind+4 {
		return nil, dataTooShort
	}
	nProcesses := uint16(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	vks := make([]*bn256.VerificationKey, nProcesses)
	for i := range vks {
		if len(data) < ind+4 {
			return nil, dataTooShort
		}
		vkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		if len(data) < ind+vkLen {
			return nil, dataTooShort
		}
		vks[i], err = new(bn256.VerificationKey).Unmarshal(data[ind:(ind + vkLen)])
		if err != nil {
			return nil, errors.New("unmarshal of vk failed")
		}
		ind += vkLen
	}
	encSKs := make([]encrypt.CipherText, nProcesses)
	for i := range encSKs {
		if len(data) < ind+4 {
			return nil, dataTooShort
		}
		skLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		if len(data) < ind+skLen {
			return nil, dataTooShort
		}
		encSKs[i] = data[ind:(ind + skLen)]
		ind += skLen
	}

	decrypted, err := decryptionKey.Decrypt(encSKs[owner])
	if err != nil {
		return nil, err
	}
	if len(decrypted) < 2 {
		return nil, dataTooShort
	}
	dealer := binary.LittleEndian.Uint16(decrypted[:2])
	if dealer != sender {
		return nil, errors.New("sender doesn't match with the dealer")
	}
	sk, err := new(bn256.SecretKey).Unmarshal(decrypted[2:])
	if err != nil {
		return nil, err
	}
	return &ThresholdCoin{
		dealer:    dealer,
		owner:     owner,
		threshold: threshold,
		globalVK:  globalVK,
		vks:       vks,
		encSKs:    encSKs,
		sk:        sk,
	}, nil
}
