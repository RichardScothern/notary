package trustmanager

import (
	"crypto/x509"
	"errors"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
)

// X509FileStore implements X509Store that persists on disk
type X509FileStore struct {
	validate       Validator
	fileMap        map[CertID]string
	fingerprintMap map[CertID]*x509.Certificate
	nameMap        map[string][]CertID
	fileStore      FileStore
}

// NewX509FileStore returns a new X509FileStore.
func NewX509FileStore(directory string) (*X509FileStore, error) {
	validate := ValidatorFunc(func(cert *x509.Certificate) bool { return true })
	return newX509FileStore(directory, validate)
}

// NewX509FilteredFileStore returns a new X509FileStore that validates certificates
// that are added.
func NewX509FilteredFileStore(directory string, validate func(*x509.Certificate) bool) (*X509FileStore, error) {
	return newX509FileStore(directory, validate)
}

func newX509FileStore(directory string, validate func(*x509.Certificate) bool) (*X509FileStore, error) {
	fileStore, err := NewSimpleFileStore(directory, certExtension)
	if err != nil {
		return nil, err
	}

	s := &X509FileStore{
		validate:       ValidatorFunc(validate),
		fileMap:        make(map[CertID]string),
		fingerprintMap: make(map[CertID]*x509.Certificate),
		nameMap:        make(map[string][]CertID),
		fileStore:      fileStore,
	}

	loadCertsFromDir(s)

	return s, nil
}

// AddCert creates a filename for a given cert and adds a certificate with that name
func (s X509FileStore) AddCert(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("adding nil Certificate to X509Store")
	}

	// Check if this certificate meets our validation criteria
	if !s.validate.Validate(cert) {
		return errors.New("certificate validation failed")
	}
	// Attempt to write the certificate to the file
	if err := s.addNamedCert(cert); err != nil {
		return err
	}

	return nil
}

// addNamedCert allows adding a certificate while controling the filename it gets
// stored under. If the file does not exist on disk, saves it.
func (s X509FileStore) addNamedCert(cert *x509.Certificate) error {
	fileName, keyID, err := fileName(cert)
	if err != nil {
		return err
	}

	logrus.Debug("Adding cert with keyID: ", keyID)
	// Validate if we already loaded this certificate before
	if _, ok := s.fingerprintMap[keyID]; ok {
		return errors.New("certificate already in the store")
	}

	// Convert certificate to PEM
	certBytes := CertToPEM(cert)

	// Save the file to disk if not already there.
	filePath, err := s.fileStore.GetPath(fileName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := s.fileStore.Add(fileName, certBytes); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// We wrote the certificate succcessfully, add it to our in-memory storage
	s.fingerprintMap[keyID] = cert
	s.fileMap[keyID] = fileName

	name := string(cert.RawSubject)
	s.nameMap[name] = append(s.nameMap[name], keyID)

	return nil
}

// RemoveCert removes a certificate from a X509FileStore.
func (s X509FileStore) RemoveCert(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("removing nil Certificate from X509Store")
	}

	keyID, err := fingerprintCert(cert)
	if err != nil {
		return err
	}
	delete(s.fingerprintMap, keyID)
	filename := s.fileMap[keyID]
	delete(s.fileMap, keyID)

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

	if err := s.fileStore.Remove(filename); err != nil {
		return err
	}

	return nil
}

// AddCertFromPEM adds the first certificate that it finds in the byte[], returning
// an error if no Certificates are found
func (s X509FileStore) AddCertFromPEM(pemBytes []byte) error {
	cert, err := LoadCertFromPEM(pemBytes)
	if err != nil {
		return err
	}
	return s.AddCert(cert)
}

// AddCertFromFile tries to adds a X509 certificate to the store given a filename
func (s X509FileStore) AddCertFromFile(filename string) error {
	cert, err := LoadCertFromFile(filename)
	if err != nil {
		return err
	}

	return s.AddCert(cert)
}

// GetCertificates returns an array with all of the current X509 Certificates.
func (s X509FileStore) GetCertificates() []*x509.Certificate {
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
func (s X509FileStore) GetCertificatePool() *x509.CertPool {
	pool := x509.NewCertPool()

	for _, v := range s.fingerprintMap {
		pool.AddCert(v)
	}
	return pool
}

// GetCertificateByKeyID returns the certificate that matches a certain keyID or error
func (s X509FileStore) GetCertificateByKeyID(keyID string) (*x509.Certificate, error) {
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
func (s X509FileStore) GetVerifyOptions(dnsName string) (x509.VerifyOptions, error) {
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

func fileName(cert *x509.Certificate) (string, CertID, error) {
	keyID, err := fingerprintCert(cert)
	if err != nil {
		return "", "", err
	}

	return path.Join(cert.Subject.CommonName, string(keyID)), keyID, nil
}
