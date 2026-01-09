package store

import (
	"bufio"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

type Store struct {
	pathname string
	f        *os.File
	c        cipher.AEAD
	mu       sync.Mutex
	closed   bool
}

func Open(dir string) (*Store, error) {
	pathname := filepath.Join(dir, "masking.db")
	sum := sha256.Sum256([]byte(pathname))
	keyPath := filepath.Join(dir, "runner"+hex.EncodeToString(sum[:]))

	_ = os.MkdirAll(filepath.Dir(pathname), 0o755)
	_, err := os.Stat(pathname)
	if err != nil {
		// store file doesn't exist, so re-generate key
		if err := os.WriteFile(keyPath, generateKey(), 0o644); err != nil {
			return nil, fmt.Errorf("writing key: %w", err)
		}
	}

	f, err := openFile(pathname)
	if err != nil {
		return nil, fmt.Errorf("opening store file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat store file: %w", err)
	}

	if info.Size() == 0 {
		if _, err := f.Write(generateKey()); err != nil {
			return nil, fmt.Errorf("writing store key: %w", err)
		}
		_, _ = f.Seek(0, io.SeekStart)
		if err := f.Sync(); err != nil {
			return nil, err
		}
	}

	key, err := deriveEncryptionKey(f, keyPath)
	if err != nil {
		return nil, fmt.Errorf("deriving key: %w", err)
	}

	c, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	return &Store{
		pathname: pathname,
		f:        f,
		c:        c,
	}, nil
}

func (s *Store) List() ([]string, error) {
	buf := bufio.NewReader(io.NewSectionReader(s.f, 32, math.MaxInt64))

	var results []string
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return results, nil
			}
			return results, err
		}

		msg, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			return results, fmt.Errorf("decoding msg: %w", err)
		}

		if len(line) < s.c.NonceSize() {
			return results, fmt.Errorf("encrypted message length too small")
		}

		nonce, ciphertext := msg[:s.c.NonceSize()], msg[s.c.NonceSize():]
		plaintext, err := s.c.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return results, fmt.Errorf("opening encrypted message: %w", err)
		}

		results = append(results, string(plaintext))
	}
}

func (s *Store) Add(phrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return os.ErrClosed
	}

	input := []byte(phrase)
	nonce := make([]byte, s.c.NonceSize(), s.c.NonceSize()+len(input)+s.c.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	line := base64.StdEncoding.EncodeToString(s.c.Seal(nonce, nonce, input, nil)) + "\n"
	if _, err := s.f.Write([]byte(line)); err != nil {
		return err
	}

	if err := s.f.Sync(); err != nil {
		return err
	}

	return nil
}

func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	s.f.Close()
}

func generateKey() []byte {
	var b [32]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	return b[:]
}

func deriveEncryptionKey(f *os.File, keyPath string) ([]byte, error) {
	var key1 [32]byte
	if _, err := io.ReadFull(f, key1[:]); err != nil {
		return nil, err
	}

	key2, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	if len(key2) < len(key1) {
		return nil, fmt.Errorf("key1 and key2 not the same size")
	}

	for i := 0; i < len(key1); i++ {
		key1[i] ^= key2[i]
	}

	return key1[:], nil
}
