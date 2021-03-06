package api

import (
	"crypto/rand"

	"github.com/agl/ed25519"
	"github.com/docker/notary/signer"
	"github.com/docker/notary/signer/keys"
	"github.com/endophage/gotuf/data"

	pb "github.com/docker/notary/proto"
)

// EdDSASigningService is an implementation of SigningService
type EdDSASigningService struct {
	KeyDB signer.KeyDatabase
}

// CreateKey creates a key and returns its public components
func (s EdDSASigningService) CreateKey() (*pb.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	k := data.NewPrivateKey(data.ED25519Key, pub[:], priv[:])

	err = s.KeyDB.AddKey(k)
	if err != nil {
		return nil, err
	}

	pubKey := &pb.PublicKey{KeyInfo: &pb.KeyInfo{KeyID: &pb.KeyID{ID: k.ID()}, Algorithm: &pb.Algorithm{Algorithm: k.Algorithm().String()}}, PublicKey: pub[:]}

	return pubKey, nil
}

// DeleteKey removes a key from the key database
func (s EdDSASigningService) DeleteKey(keyID *pb.KeyID) (*pb.Void, error) {
	return s.KeyDB.DeleteKey(keyID)
}

// KeyInfo returns the public components of a particular key
func (s EdDSASigningService) KeyInfo(keyID *pb.KeyID) (*pb.PublicKey, error) {
	return s.KeyDB.KeyInfo(keyID)
}

// Signer returns a Signer for a specific KeyID
func (s EdDSASigningService) Signer(keyID *pb.KeyID) (signer.Signer, error) {
	key, err := s.KeyDB.GetKey(keyID)
	if err != nil {
		return nil, keys.ErrInvalidKeyID
	}
	return &Ed25519Signer{privateKey: key}, nil
}

// NewEdDSASigningService returns an instance of KeyDB
func NewEdDSASigningService(keyDB signer.KeyDatabase) *EdDSASigningService {
	return &EdDSASigningService{
		KeyDB: keyDB,
	}
}
