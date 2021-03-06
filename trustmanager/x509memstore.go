package trustmanager

import (
	"crypto/x509"
	"errors"
)

// X509MemStore implements X509Store as an in-memory object with no persistence
type X509MemStore struct {
	validate       Validator
	fingerprintMap map[CertID]*x509.Certificate
	nameMap        map[string][]CertID
}

// NewX509MemStore returns a new X509MemStore.
func NewX509MemStore() *X509MemStore {
	validate := ValidatorFunc(func(cert *x509.Certificate) bool { return true })

	return &X509MemStore{
		validate:       validate,
		fingerprintMap: make(map[CertID]*x509.Certificate),
		nameMap:        make(map[string][]CertID),
	}
}

// NewX509FilteredMemStore returns a new X509FileStore that validates certificates
// that are added.
func NewX509FilteredMemStore(validate func(*x509.Certificate) bool) *X509MemStore {
	s := &X509MemStore{

		validate:       ValidatorFunc(validate),
		fingerprintMap: make(map[CertID]*x509.Certificate),
		nameMap:        make(map[string][]CertID),
	}

	return s
}

// AddCert adds a certificate to the store
func (s X509MemStore) AddCert(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("adding nil Certificate to X509Store")
	}

	if !s.validate.Validate(cert) {
		return errors.New("certificate failed validation")
	}

	keyID, err := fingerprintCert(cert)
	if err != nil {
		return err
	}

	s.fingerprintMap[keyID] = cert
	name := string(cert.RawSubject)
	s.nameMap[name] = append(s.nameMap[name], keyID)

	return nil
}

// RemoveCert removes a certificate from a X509MemStore.
func (s X509MemStore) RemoveCert(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("removing nil Certificate to X509Store")
	}

	keyID, err := fingerprintCert(cert)
	if err != nil {
		return err
	}
	delete(s.fingerprintMap, keyID)
	name := string(cert.RawSubject)

	// Filter the fingerprint out of this name entry
	fpList := s.nameMap[name]
	newfpList := fpList[:0]
	for _, x := range fpList {
		if x != keyID {
			newfpList = append(newfpList, x)
		}
	}

	s.nameMap[name] = newfpList
	return nil
}

// AddCertFromPEM adds a certificate to the store from a PEM blob
func (s X509MemStore) AddCertFromPEM(pemBytes []byte) error {
	cert, err := LoadCertFromPEM(pemBytes)
	if err != nil {
		return err
	}
	return s.AddCert(cert)
}

// AddCertFromFile tries to adds a X509 certificate to the store given a filename
func (s X509MemStore) AddCertFromFile(originFilname string) error {
	cert, err := LoadCertFromFile(originFilname)
	if err != nil {
		return err
	}

	return s.AddCert(cert)
}

// GetCertificates returns an array with all of the current X509 Certificates.
func (s X509MemStore) GetCertificates() []*x509.Certificate {
	certs := make([]*x509.Certificate, len(s.fingerprintMap))
	i := 0
	for _, v := range s.fingerprintMap {
		certs[i] = v
		i++
	}
	return certs
}

// GetCertificatePool returns an x509 CertPool loaded with all the certificates
// in the store.
func (s X509MemStore) GetCertificatePool() *x509.CertPool {
	pool := x509.NewCertPool()

	for _, v := range s.fingerprintMap {
		pool.AddCert(v)
	}
	return pool
}

// GetCertificateByKeyID returns the certificate that matches a certain keyID or error
func (s X509MemStore) GetCertificateByKeyID(keyID string) (*x509.Certificate, error) {
	// If it does not look like a hex encoded sha256 hash, error
	if len(keyID) != 64 {
		return nil, errors.New("invalid Subject Key Identifier")
	}

	// Check to see if this subject key identifier exists
	if cert, ok := s.fingerprintMap[CertID(keyID)]; ok {
		return cert, nil

	}
	return nil, errors.New("certificate not found in Key Store")
}

// GetVerifyOptions returns VerifyOptions with the certificates within the KeyStore
// as part of the roots list. This never allows the use of system roots, returning
// an error if there are no root CAs.
func (s X509MemStore) GetVerifyOptions(dnsName string) (x509.VerifyOptions, error) {
	// If we have no Certificates loaded return error (we don't want to rever to using
	// system CAs).
	if len(s.fingerprintMap) == 0 {
		return x509.VerifyOptions{}, errors.New("no root CAs available")
	}

	opts := x509.VerifyOptions{
		DNSName: dnsName,
		Roots:   s.GetCertificatePool(),
	}

	return opts, nil
}
