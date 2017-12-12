package p2p2

import (
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
)

type Key interface {
	String() string // this is a base58 encoded of Bytes()
	Bytes() []byte  // raw key binary data - 32 bytes, big endian
	Pretty() string // pretty print key id
}

type PrivateKey interface {
	Key

	GetPublicKey() PublicKey // get the pub key corresponding to this priv key
	Sign([]byte) ([]byte, error)

	// Decrypt binary data encrypted with the public key of this private key
	Decrypt(in []byte) ([]byte, error)

	InternalKey() *btcec.PrivateKey
}

type PublicKey interface { // 33 bytes
	Key
	Verify(data []byte, sig []byte) (bool, error)

	// encrypt data so it is only decryptable w the private key of this key
	Encrypt(in []byte) ([]byte, error)

	InternalKey() *btcec.PublicKey
}

////////////////////////////////////////////////////////

type publicKeyImpl struct {
	k *btcec.PublicKey
}

type privateKeyImpl struct {
	k *btcec.PrivateKey
}

func GenerateKeyPair() (PrivateKey, PublicKey, error) {
	privKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return nil, nil, err
	}

	return &privateKeyImpl{privKey}, &publicKeyImpl{privKey.PubKey()}, nil
}

func NewPrivateKey(data []byte) PrivateKey {
	privk, _ := btcec.PrivKeyFromBytes(btcec.S256(), data)
	return &privateKeyImpl{privk}
}

func NewPrivateKeyFromString(s string) PrivateKey {
	data := b58.Decode(s)
	return NewPrivateKey(data)
}

func (p *privateKeyImpl) InternalKey() *btcec.PrivateKey {
	return p.k
}

func (p *privateKeyImpl) Bytes() []byte {
	return p.k.Serialize()
}

func (p *privateKeyImpl) String() string {
	bytes := p.Bytes()
	return b58.Encode(bytes)
}

func (p *privateKeyImpl) GetPublicKey() PublicKey {
	pubKey := p.k.PubKey()
	return &publicKeyImpl{k: pubKey}
}

func (p *privateKeyImpl) Pretty() string {
	pstr := p.String()
	maxRunes := 6
	if len(pstr) < maxRunes {
		maxRunes = len(pstr)
	}
	return fmt.Sprintf("<PrivKey %s>", pstr[:maxRunes])
}

func (p *privateKeyImpl) Sign(in []byte) ([]byte, error) {
	signature, err := p.k.Sign(in)
	if err != nil {
		return nil, err
	}
	return signature.Serialize(), nil
}

func (p *privateKeyImpl) Decrypt(in []byte) ([]byte, error) {
	return btcec.Decrypt(p.k, in)
}

////////////////////////////////////////

// data - binary key data
func NewPublicKey(data []byte) (PublicKey, error) {
	k, err := btcec.ParsePubKey(data, btcec.S256())
	if err != nil {
		return nil, err
	}

	return &publicKeyImpl{k}, nil
}

func NewPublicKeyFromString(s string) (PublicKey, error) {
	data := b58.Decode(s)
	return NewPublicKey(data)
}

func (p *publicKeyImpl) InternalKey() *btcec.PublicKey {
	return p.k
}


func (p *publicKeyImpl) Bytes() []byte {
	return p.k.SerializeCompressed()
}

func (p *publicKeyImpl) String() string {
	return b58.Encode(p.Bytes())
}

func (p *publicKeyImpl) Pretty() string {
	pstr := p.String()
	maxRunes := 6
	if len(pstr) < maxRunes {
		maxRunes = len(pstr)
	}
	return fmt.Sprintf("<PubKey %s>", pstr[:maxRunes])
}

func (p *publicKeyImpl) Verify(data []byte, sig []byte) (bool, error) {
	signature, err := btcec.ParseSignature(sig, btcec.S256())
	if err != nil {
		return false, err
	}

	verified := signature.Verify(data, p.k)
	return verified, nil
}

// Encrypt data that can only be decrypted by the private key of this key
func (p *publicKeyImpl) Encrypt(in []byte) ([]byte, error) {
	return btcec.Encrypt(p.k, in)
}
