package api

import (
	"fmt"
	"log"

	"github.com/docker/notary/signer"
	"github.com/docker/notary/signer/keys"
	"github.com/endophage/gotuf/data"
	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	pb "github.com/docker/notary/proto"
)

//KeyManagementServer implements the KeyManagementServer grpc interface
type KeyManagementServer struct {
	SigServices signer.SigningServiceIndex
}

//SignerServer implements the SignerServer grpc interface
type SignerServer struct {
	SigServices signer.SigningServiceIndex
}

//CreateKey returns a PublicKey created using KeyManagementServer's SigningService
func (s *KeyManagementServer) CreateKey(ctx context.Context, algorithm *pb.Algorithm) (*pb.PublicKey, error) {
	service := s.SigServices[data.KeyAlgorithm(algorithm.Algorithm)]

	if service == nil {
		return nil, fmt.Errorf("algorithm %s not supported for create key", algorithm.Algorithm)
	}

	key, err := service.CreateKey()
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Key creation failed")
	}
	log.Println("[Notary-signer CreateKey] : Created KeyID ", key.KeyInfo.KeyID.ID)
	return key, nil
}

//DeleteKey deletes they key associated with a KeyID
func (s *KeyManagementServer) DeleteKey(ctx context.Context, keyID *pb.KeyID) (*pb.Void, error) {
	_, service, err := FindKeyByID(s.SigServices, keyID)

	if err != nil {
		return nil, grpc.Errorf(codes.NotFound, "Invalid keyID: key %s not found", keyID.ID)
	}

	_, err = service.DeleteKey(keyID)
	log.Println("[Notary-signer DeleteKey] : Deleted KeyID ", keyID.ID)
	if err != nil {
		switch err {
		case keys.ErrInvalidKeyID:
			return nil, grpc.Errorf(codes.NotFound, "Invalid keyID: key %s not found", keyID.ID)
		default:
			return nil, grpc.Errorf(codes.Internal, "Key deletion for keyID %s failed", keyID.ID)
		}
	}

	return &pb.Void{}, nil
}

//GetKeyInfo returns they PublicKey associated with a KeyID
func (s *KeyManagementServer) GetKeyInfo(ctx context.Context, keyID *pb.KeyID) (*pb.PublicKey, error) {
	_, service, err := FindKeyByID(s.SigServices, keyID)

	if err != nil {
		return nil, grpc.Errorf(codes.NotFound, "Invalid keyID: key %s not found", keyID.ID)
	}

	key, err := service.KeyInfo(keyID)
	if err != nil {
		return nil, grpc.Errorf(codes.NotFound, "Invalid keyID: key %s not found", keyID.ID)
	}
	log.Println("[Notary-signer GetKeyInfo] : Returning PublicKey for KeyID ", keyID.ID)
	return key, nil
}

//Sign signs a message and returns the signature using a private key associate with the KeyID from the SignatureRequest
func (s *SignerServer) Sign(ctx context.Context, sr *pb.SignatureRequest) (*pb.Signature, error) {
	_, service, err := FindKeyByID(s.SigServices, sr.KeyID)

	if err != nil {
		return nil, grpc.Errorf(codes.NotFound, "Invalid keyID: key %s not found", sr.KeyID.ID)
	}

	log.Println("[Notary-signer Sign] : Signing ", string(sr.Content), " with KeyID ", sr.KeyID.ID)
	signer, err := service.Signer(sr.KeyID)
	if err == keys.ErrInvalidKeyID {
		return nil, grpc.Errorf(codes.NotFound, "Invalid keyID: key %s not found", sr.KeyID.ID)
	} else if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Signing failed for keyID %s on hash %s", sr.KeyID.ID, sr.Content)
	}

	signature, err := signer.Sign(sr)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Signing failed for keyID %s on hash %s", sr.KeyID.ID, sr.Content)
	}

	return signature, nil
}
