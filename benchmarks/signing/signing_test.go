package signing

import (
	"math/rand"
	"testing"
	"time"

	godium "go.artemisc.eu/godium"
	"go.artemisc.eu/godium/random"
	godium_sign "go.artemisc.eu/godium/sign"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/nacl/sign"
)

const (
	testDataSize  int = 1024 * 1024
	signatureSize int = 64
)

type signature []byte

type signedMessage []byte

type privateKey interface {
	signDetached(data []byte, signature signature) signature
	sign(dst, data []byte) signedMessage
}

type publicKey interface {
	verify(dst, signed []byte) bool
	verifyDetached(data []byte, signature signature) bool
}

type naclPublicKeyData *[32]byte

type naclPublicKey struct {
	data naclPublicKeyData
}

type naclPrivateKeyData *[64]byte

type naclPrivateKey struct {
	data naclPrivateKeyData
}

func (sk *naclPrivateKey) sign(dst, data []byte) signedMessage {
	return sign.Sign(dst[:0], data, sk.data)
}

func (sk *naclPrivateKey) signDetached(data []byte, signature signature) signature {
	return sign.Sign(nil, data, sk.data)[:sign.Overhead]
}

func (pk *naclPublicKey) verify(dst, signed []byte) bool {
	_, v := sign.Open(dst[:0], signed, pk.data)
	return v
}

func (pk *naclPublicKey) verifyDetached(data []byte, signature signature) bool {
	msgSig := append(signature, data...)
	_, v := sign.Open(nil, msgSig, pk.data)
	return v
}

func generateNaclKeys() (publicKey, privateKey, error) {
	pubData, privData, err := sign.GenerateKey(nil)
	if err != nil {
		return nil, nil, err
	}

	pub := naclPublicKey{pubData}
	priv := naclPrivateKey{privData}

	return &pub, &priv, nil
}

type ed25519PublicKey struct {
	data ed25519.PublicKey
}

type ed25519PrivateKey struct {
	data ed25519.PrivateKey
}

func (sk *ed25519PrivateKey) sign(dst, data []byte) signedMessage {
	signature := ed25519.Sign(sk.data, data)
	copy(dst, signature)
	copy(dst[ed25519.SignatureSize:], data)
	return dst
}

func (sk *ed25519PrivateKey) signDetached(data []byte, signature signature) signature {
	return ed25519.Sign(sk.data, data)
}

func (pk *ed25519PublicKey) verify(dst, signed []byte) bool {
	result := ed25519.Verify(pk.data, signed[ed25519.SignatureSize:], signed[:ed25519.SignatureSize])
	copy(dst, signed[ed25519.SignatureSize:])
	return result
}

func (pk *ed25519PublicKey) verifyDetached(data []byte, signature signature) bool {
	return ed25519.Verify(pk.data, data, signature)
}

func generateEd25519Keys() (publicKey, privateKey, error) {
	pk, sk, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, err
	}
	return &ed25519PublicKey{pk}, &ed25519PrivateKey{sk}, nil
}

type godiumPrivateKey struct {
	data godium.Sign
}

type godiumPublicKey struct {
	data godium.SignVerifier
}

func (sk *godiumPrivateKey) sign(dst, data []byte) signedMessage {
	return sk.data.Sign(dst, data)
}

func (sk *godiumPrivateKey) signDetached(data []byte, signature signature) signature {
	return sk.data.SignDetached(signature[:0], data)
}

func (pk *godiumPublicKey) verify(dst, signed []byte) bool {
	_, v := pk.data.Open(dst[:0], signed)
	return v
}

func (pk *godiumPublicKey) verifyDetached(data []byte, signature signature) bool {
	return pk.data.VerifyDetached(signature, data)
}

func generateGodiumKeys() (publicKey, privateKey, error) {
	random := random.New()
	sign, err := godium_sign.KeyPairEd25519(random)
	if err != nil {
		return nil, nil, err
	}
	return &godiumPublicKey{godium_sign.NewEd25519Verifier(sign.PublicKey())}, &godiumPrivateKey{sign}, nil
}

func initializeData() (dst []byte, data []byte) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	var dataArray [2*testDataSize + signatureSize]byte
	rand.Read(dataArray[:])
	dst = dataArray[testDataSize:]
	data = dataArray[:testDataSize]
	return
}

func BenchmarkVerifyNacl(b *testing.B) {
	dst, testData := initializeData()
	pk, sk, err := generateNaclKeys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkVerify(b, pk, sk, dst, testData)
}

func BenchmarkSignNacl(b *testing.B) {
	dst, testData := initializeData()
	_, sk, err := generateNaclKeys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkSign(b, sk, dst, testData)
}

func BenchmarkSignAndVerifyNacl(b *testing.B) {
	dst, testData := initializeData()
	pk, sk, err := generateNaclKeys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkSignAndVerify(b, pk, sk, dst, testData)
}

func BenchmarkVerifyEd25519(b *testing.B) {
	dst, testData := initializeData()
	pk, sk, err := generateEd25519Keys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkVerify(b, pk, sk, dst, testData)
}

func BenchmarkSignEd25519(b *testing.B) {
	dst, testData := initializeData()
	_, sk, err := generateEd25519Keys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkSign(b, sk, dst, testData)
}

func BenchmarkSignAndVerifyEd25519(b *testing.B) {
	dst, testData := initializeData()
	pk, sk, err := generateEd25519Keys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkSignAndVerify(b, pk, sk, dst, testData)
}

func BenchmarkSignGodium(b *testing.B) {
	dst, testData := initializeData()
	_, sk, err := generateGodiumKeys()
	if err != nil {
		b.Fatal("unable to generate keys")
	}
	benchmarkSign(b, sk, dst, testData)
}

// TODO Missing 'verify' and 'sign and verify' benchmarks for godium.

func benchmarkVerify(b *testing.B, pk publicKey, sk privateKey, dst, data []byte) {
	signature := sk.signDetached(data, dst)

	b.Run("verify detached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !pk.verifyDetached(data, signature) {
				b.Fatal("failed to verify")
			}
		}
	})

	signed := sk.sign(dst, data)

	b.Run("verify", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !pk.verify(data, signed) {
				b.Fatal("failed to verify")
			}
		}
	})
}

func benchmarkSign(b *testing.B, sk privateKey, dst, data []byte) {

	b.Run("sign", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if len(sk.sign(dst, data)) == 0 {
				b.Fatal("failed to sign")
			}
		}
	})

	b.Run("sign detached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if len(sk.signDetached(data, dst)) == 0 {
				b.Fatal("failed to sign")
			}
		}
	})
}

func benchmarkSignAndVerify(b *testing.B, pk publicKey, sk privateKey, dst, data []byte) {

	b.Run("sign and verify", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			signed := sk.sign(dst, data)
			if len(signed) == 0 {
				b.Fatal("failed to sign")
			}
			if !pk.verify(data, signed) {
				b.Fatal("failed to verify")
			}
		}
	})

	b.Run("sign and verify detached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			signature := sk.signDetached(data, dst)
			if len(signature) == 0 {
				b.Fatal("failed to sign")
			}
			if !pk.verifyDetached(data, signature) {
				b.Fatal("failed to verify")
			}
		}
	})
}
