package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// subjectStrictRegistry rejects manifests whose subject is not pushed yet,
// like GitLab's container registry does.
type subjectStrictRegistry struct {
	next      http.Handler
	mu        sync.Mutex
	manifests map[string]bool
	rejected  []string

	// subjectSeen, when set, is closed on the first manifest PUT that has a
	// subject; manifest PUTs without one wait for it. This makes the
	// unordered push lose the race regardless of scheduling.
	subjectSeen     chan struct{}
	subjectSeenOnce sync.Once
}

func newSubjectStrictRegistry() *subjectStrictRegistry {
	return &subjectStrictRegistry{
		next:      registry.New(registry.Logger(log.New(io.Discard, "", 0))),
		manifests: map[string]bool{},
	}
}

func (s *subjectStrictRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPut || !strings.Contains(req.URL.Path, "/manifests/") {
		s.next.ServeHTTP(w, req)
		return
	}

	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(body))

	var m struct {
		Subject *v1.Descriptor `json:"subject"`
	}
	_ = json.Unmarshal(body, &m)

	dgst, _, _ := v1.SHA256(bytes.NewReader(body))

	if s.subjectSeen != nil {
		if m.Subject != nil {
			s.subjectSeenOnce.Do(func() { close(s.subjectSeen) })
		} else {
			select {
			case <-s.subjectSeen:
			case <-time.After(5 * time.Second):
			}
		}
	}

	s.mu.Lock()
	subjectMissing := m.Subject != nil && !s.manifests[m.Subject.Digest.String()]
	if subjectMissing {
		s.rejected = append(s.rejected, dgst.String())
	}
	s.mu.Unlock()

	if subjectMissing {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"code":"MANIFEST_BLOB_UNKNOWN","message":"blob unknown to registry"}]}`))
		return
	}

	rec := httptest.NewRecorder()
	s.next.ServeHTTP(rec, req)
	maps.Copy(w.Header(), rec.Header())
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())

	if rec.Code/100 != 2 {
		return
	}

	s.mu.Lock()
	s.manifests[dgst.String()] = true
	s.mu.Unlock()
}

// buildxLayout mimics buildx multi-platform output with provenance:
// index.json -> nested index -> platform images + attestation manifests.
func buildxLayout(t *testing.T, dir string, platforms int) v1.ImageIndex {
	t.Helper()

	var adds []mutate.IndexAddendum
	for range platforms {
		img, err := random.Image(1<<20, 2)
		if err != nil {
			t.Fatal(err)
		}
		img = mutate.MediaType(img, types.OCIManifestSchema1)
		img = mutate.ConfigMediaType(img, types.OCIConfigJSON)
		d, err := img.Digest()
		if err != nil {
			t.Fatal(err)
		}
		sz, err := img.Size()
		if err != nil {
			t.Fatal(err)
		}
		adds = append(adds, mutate.IndexAddendum{Add: img})

		attLayer, err := random.Layer(512, types.MediaType("application/vnd.in-toto+json"))
		if err != nil {
			t.Fatal(err)
		}
		att := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
		att = mutate.ConfigMediaType(att, types.OCIConfigJSON)
		att, err = mutate.AppendLayers(att, attLayer)
		if err != nil {
			t.Fatal(err)
		}
		att = mutate.Subject(att, v1.Descriptor{MediaType: types.OCIManifestSchema1, Digest: d, Size: sz}).(v1.Image)
		adds = append(adds, mutate.IndexAddendum{
			Add: att,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: "unknown", Architecture: "unknown"},
				Annotations: map[string]string{
					"vnd.docker.reference.type":   "attestation-manifest",
					"vnd.docker.reference.digest": d.String(),
				},
			},
		})
	}

	inner := mutate.IndexMediaType(mutate.AppendManifests(empty.Index, adds...), types.OCIImageIndex)
	top := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: inner})

	if _, err := layout.Write(dir, top); err != nil {
		t.Fatal(err)
	}
	idx, err := layout.ImageIndexFromPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestSubjectlessManifests(t *testing.T) {
	idx := buildxLayout(t, t.TempDir(), 3)

	got, err := subjectlessManifests(idx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 subjectless manifests, got %d", len(got))
	}
	for d, img := range got {
		m, err := img.Manifest()
		if err != nil {
			t.Fatal(err)
		}
		if m.Subject != nil {
			t.Errorf("%s has a subject", d)
		}
		if len(m.Layers) != 2 {
			t.Errorf("%s is not a platform image, has %d layers", d, len(m.Layers))
		}
		if id, _ := img.Digest(); id != d {
			t.Errorf("key %s does not match image digest %s", d, id)
		}
	}
}

func TestPushSubjectsFirst(t *testing.T) {
	idx := buildxLayout(t, t.TempDir(), 3)

	reg := newSubjectStrictRegistry()
	srv := httptest.NewServer(reg)
	defer srv.Close()

	repo, err := name.NewRepository(srv.Listener.Addr().String() + "/dev")
	if err != nil {
		t.Fatal(err)
	}
	ref := repo.Tag("ubuntu-abc123")
	ctx := t.Context()

	pusher, err := remote.NewPusher()
	if err != nil {
		t.Fatal(err)
	}

	if err := pushSubjectsFirst(ctx, pusher, repo, idx); err != nil {
		t.Fatalf("pushSubjectsFirst: %v", err)
	}
	if err := pusher.Push(ctx, ref, idx); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(reg.rejected) != 0 {
		t.Fatalf("registry rejected %d manifests with missing subject: %v", len(reg.rejected), reg.rejected)
	}

	// The nested index and all six children must exist on the remote.
	top, err := remote.Index(ref)
	if err != nil {
		t.Fatal(err)
	}
	tm, err := top.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(tm.Manifests) != 1 {
		t.Fatalf("expected 1 nested index on remote, got %d", len(tm.Manifests))
	}
	got, err := top.ImageIndex(tm.Manifests[0].Digest)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := got.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(gm.Manifests) != 6 {
		t.Fatalf("expected 6 children on remote, got %d", len(gm.Manifests))
	}
	for _, d := range gm.Manifests {
		if _, err := remote.Head(repo.Digest(d.Digest.String())); err != nil {
			t.Errorf("child %s missing on remote: %v", d.Digest, err)
		}
	}
}

// If this starts passing, go-containerregistry orders children itself and
// pushSubjectsFirst can go.
func TestPush_WithoutSubjectsFirst_IsRejectedBySubjectStrictRegistry(t *testing.T) {
	idx := buildxLayout(t, t.TempDir(), 3)

	reg := newSubjectStrictRegistry()
	reg.subjectSeen = make(chan struct{})
	srv := httptest.NewServer(reg)
	defer srv.Close()

	repo, err := name.NewRepository(srv.Listener.Addr().String() + "/dev")
	if err != nil {
		t.Fatal(err)
	}
	pusher, err := remote.NewPusher()
	if err != nil {
		t.Fatal(err)
	}

	err = pusher.Push(t.Context(), repo.Tag("ubuntu-abc123"), idx)
	if err == nil || !strings.Contains(err.Error(), "MANIFEST_BLOB_UNKNOWN") {
		t.Fatalf("expected MANIFEST_BLOB_UNKNOWN from subject-strict registry, got: %v", err)
	}
}
