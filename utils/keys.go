package utils

import (
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/hw09234/gm-crypto/sm2"
	"github.com/hw09234/gm-crypto/x509"
)

var (
	oidNamedCurveSm2  = asn1.ObjectIdentifier{1, 2, 156, 10197, 1, 301}
	oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
)

// struct to hold info required for PKCS#8
type pkcs8Info struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}

func PrivateKeyToDER(privateKey *sm2.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, errors.New("invalid ecdsa private key. It must be different from nil")
	}

	return x509.MarshalECPrivateKey(privateKey)
}

// PrivateKeyToPEM converts the private key to PEM format.
// EC private keys are converted to PKCS#8 format.
// RSA private keys are converted to PKCS#1 format.
func PrivateKeyToPEM(privateKey *sm2.PrivateKey, pwd []byte) ([]byte, error) {
	if len(pwd) != 0 {
		return PrivateKeyToEncryptedPEM(privateKey, pwd)
	}
	if privateKey == nil {
		return nil, errors.New("Invalid key. It must be different from nil.")
	}

	privateKeyBytes := privateKey.D.Bytes()
	paddedPrivateKey := make([]byte, (privateKey.Curve.Params().N.BitLen()+7)/8)
	copy(paddedPrivateKey[len(paddedPrivateKey)-len(privateKeyBytes):], privateKeyBytes)
	// omit NamedCurveOID for compatibility as it's optional
	asn1Bytes, err := asn1.Marshal(ecPrivateKey{
		Version:    1,
		PrivateKey: paddedPrivateKey,
		PublicKey:  asn1.BitString{Bytes: elliptic.Marshal(privateKey.Curve, privateKey.X, privateKey.Y)},
	})

	if err != nil {
			return nil, fmt.Errorf("error marshaling SM2 key to asn1 [%s]", err)
		}
	var pkcs8Key pkcs8Info
	pkcs8Key.Version = 0
	pkcs8Key.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 2)
	pkcs8Key.PrivateKeyAlgorithm[0] = oidPublicKeyECDSA
	pkcs8Key.PrivateKeyAlgorithm[1] = oidNamedCurveSm2
	pkcs8Key.PrivateKey = asn1Bytes

	pkcs8Bytes, err := asn1.Marshal(pkcs8Key)
	if err != nil {
			return nil, fmt.Errorf("error marshaling EC key to asn1 [%s]", err)
		}
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: pkcs8Bytes,
		},
	), nil
}

// PrivateKeyToEncryptedPEM converts a private key to an encrypted PEM
func PrivateKeyToEncryptedPEM(priKey *sm2.PrivateKey, pwd []byte) ([]byte, error) {
	if priKey == nil {
		return nil, errors.New("Invalid private key. It must be different from nil.")
	}

	oid := oidNamedCurveSm2
	privateKeyBytes := priKey.D.Bytes()
	paddedPrivateKey := make([]byte, (priKey.Curve.Params().N.BitLen()+7)/8)
	copy(paddedPrivateKey[len(paddedPrivateKey)-len(privateKeyBytes):], privateKeyBytes)
	raw, err := asn1.Marshal(ecPrivateKey{
		Version:       1,
		PrivateKey:    paddedPrivateKey,
		NamedCurveOID: oid,
		PublicKey:     asn1.BitString{Bytes: elliptic.Marshal(priKey.Curve, priKey.X, priKey.Y)},
	})
	if err != nil {
		return nil, err
	}
	block, err := x509.EncryptPEMBlock(
		rand.Reader,
		"PRIVATE KEY",
		raw,
		pwd,
		x509.PEMCipherAES256)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(block), nil
}



// PEMtoPrivateKey unmarshal a pem to private key
func PEMtoPrivateKey(raw []byte, pwd []byte) (*sm2.PrivateKey, error) {
	if len(raw) == 0 {
		return nil, errors.New("invalid PEM. It must be different from nil")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("failed decoding PEM. Block must be different from nil. [% x]", raw)
	}

	if x509.IsEncryptedPEMBlock(block) {
		if len(pwd) == 0 {
			return nil, errors.New("encrypted Key. Need a password")
		}

		decrypted, err := x509.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, fmt.Errorf("failed PEM decryption [%s]", err)
		}

		key, err := x509.ParsePKCS8PrivateKey(decrypted)
		if err != nil {
			return nil, err
		}
		sm2Key, ok := key.(*sm2.PrivateKey)
		if ok {
			return sm2Key, nil
		} else {
			return nil, errors.New("key type error")
		}
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	sm2Key, ok := key.(*sm2.PrivateKey)
	if ok {
		return sm2Key, nil
	} else {
		return nil, errors.New("key type error")
	}
}

// PublicKeyToPEM marshals a public key to the pem format
func PublicKeyToPEM(publicKey *sm2.PublicKey, pwd []byte) ([]byte, error) {
	if len(pwd) != 0 {
		return PublicKeyToEncryptedPEM(publicKey, pwd)
	}

	if publicKey == nil {
		return nil, errors.New("invalid public key. It must be different from nil")
	}

	PubASN1, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: PubASN1,
		},
	), nil

}

// PublicKeyToEncryptedPEM converts a public key to encrypted pem
func PublicKeyToEncryptedPEM(publicKey *sm2.PublicKey, pwd []byte) ([]byte, error) {
	if publicKey == nil {
		return nil, errors.New("invalid public key. It must be different from nil")
	}
	if len(pwd) == 0 {
		return nil, errors.New("invalid password. It must be different from nil")
	}

	raw, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	block, err := x509.EncryptPEMBlock(
		rand.Reader,
		"PUBLIC KEY",
		raw,
		pwd,
		x509.PEMCipherAES256)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(block), nil
}

// PEMtoPublicKey unmarshal a pem to public key
func PEMtoPublicKey(raw []byte, pwd []byte) (*sm2.PublicKey, error) {
	if len(raw) == 0 {
		return nil, errors.New("invalid PEM. It must be different from nil")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("failed decoding. Block must be different from nil. [% x]", raw)
	}

	// TODO: derive from header the type of the key
	if x509.IsEncryptedPEMBlock(block) {
		if len(pwd) == 0 {
			return nil, errors.New("encrypted Key. Password must be different from nil")
		}

		decrypted, err := x509.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, fmt.Errorf("failed PEM decryption. [%s]", err)
		}

		key, err := x509.ParsePKIXPublicKey(decrypted)
		if err != nil {
			return nil, err
		}
		sm2Pk, ok := key.(*sm2.PublicKey)
		if ok {
			return sm2Pk, nil
		} else {
			return nil, errors.New("invalid public key format")
		}
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	sm2Pk, ok := key.(*sm2.PublicKey)
	if ok {
		return sm2Pk, nil
	} else {
		return nil, errors.New("invalid public key format")
	}
}
