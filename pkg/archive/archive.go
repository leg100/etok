package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/util/path"
	"k8s.io/klog/v2"
)

// Archive represents the bundle of terraform configuration to be uploaded.
type archive struct {
	// Absolute path to root module on client
	root string
	// Absolute paths to local modules on client, including root module
	mods []string
	// Base is the path of the root of the git repository containing the root
	// module
	base string
	// Maximum permitted size of compressed archive
	maxSize int64
}

// Create a new archive, with the root module 'root', based on the root of the
// git repo, 'base'.
func NewArchive(root, base string, opts ...func(*archive)) (*archive, error) {
	root, err := path.EnsureAbs(root)
	if err != nil {
		return nil, err
	}

	base, err = path.EnsureAbs(base)
	if err != nil {
		return nil, err
	}

	arc := &archive{
		root:    root,
		mods:    []string{root},
		base:    base,
		maxSize: MaxConfigSize,
	}

	for _, o := range opts {
		o(arc)
	}

	return arc, nil
}

func MaxSize(size int64) func(*archive) {
	return func(a *archive) {
		a.maxSize = size
	}
}

// Walk returns a list of local modules starting with the root module, including
// those called from the root module, directly and indirectly.
func (a *archive) Walk() error {
	mods, err := walk(a.root)
	if err != nil {
		return err
	}
	// Add modules to archive's modules
	a.mods = append(a.mods, mods...)

	return nil
}

// Archive creates a compressed tarball containing not only the root module but
// local module calls too, including transitive calls. Returns the contents of
// the tarball and the relative path to the root module within the tarball.

// Pack creates a gzipped tarball. The paths are expected to be relative to
// to the base directory. The paths are walked recursively for files and
// subdirectories, which are either added to the tarball, or ignored accordingly
// to a ruleset. During creation if the size of the tarball exceeds maxSize an
// error is returned.
func (a *archive) Pack(w io.Writer) (*Meta, error) {
	// tar > gzip > max size watcher > buf
	mw := NewMaxWriter(w, a.maxSize)
	zw := gzip.NewWriter(mw)
	tw := tar.NewWriter(zw)

	// Create an ignore rule matcher. Parses .terraformignore if exists.
	ruleMatcher := newRuleMatcher(a.base)

	// Track the metadata details as we go.
	meta := &Meta{}

	// Remove nested modules (they're walked recursively so we want to avoid
	// walking paths more than once)
	unnested := path.RemoveNestedPaths(a.mods)

	// Walk directory trees
	for _, path := range unnested {
		err := filepath.Walk(path, packWalkFn(a.base, path, path, tw, meta, true, ruleMatcher))
		if err != nil {
			return nil, err
		}
	}

	// Flush tar writer
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Flush gzip writer
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Record number of compressed bytes written
	meta.CompressedSize = mw.tally

	return meta, nil
}

// packWalkFn returns a walker func that archives a terraform module. Base is
// the base directory of the archive, src and dst are expected to be set to the
// path of the module (src represents the path on the local filesystem, whereas
// dst represents the path in the archive).  If a symlink is encountered and it
// points to a path within the module then the symlink is archived as-is.  If
// the symlink points to a path outside the module then depending upon whether
// it is a file or a directory different behaviour is followed: for a file, the
// symlink is dereferenced; for a directory, the func recurses into the target
// directory, setting src to the target and dst to the source of the symlink.
func packWalkFn(base, src, dst string, tarW *tar.Writer, meta *Meta, dereference bool, matcher *ruleMatcher) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		matched, err := matcher.match(path, info.IsDir())
		if err != nil {
			return fmt.Errorf("failed to match path %s against ignore rules", path)
		}
		if matched {
			klog.V(2).Infof("ignoring path %s", path)
			return nil
		}

		subpath, err := filepath.Rel(base, strings.Replace(path, src, dst, 1))
		if err != nil {
			return fmt.Errorf("failed to get relative path for file %q: %w", path, err)
		}
		if subpath == "." {
			return nil
		}

		// Check the file type and if we need to write the body.
		keepFile, writeBody := checkFileMode(info.Mode())
		if !keepFile {
			return nil
		}

		fm := info.Mode()
		header := &tar.Header{
			Name:    filepath.ToSlash(subpath),
			ModTime: info.ModTime(),
			Mode:    int64(fm.Perm()),
		}

		switch {
		case info.IsDir():
			header.Typeflag = tar.TypeDir
			header.Name += "/"

		case fm.IsRegular():
			header.Typeflag = tar.TypeReg
			header.Size = info.Size()

		case fm&os.ModeSymlink != 0:
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("failed to get symbolic link destination for %q: %w", path, err)
			}

			// If the target is within the current source, we
			// create the symlink using a relative path.
			if strings.Contains(target, src) {
				link, err := filepath.Rel(filepath.Dir(path), target)
				if err != nil {
					return fmt.Errorf("failed to get relative path for symlink destination %q: %w", target, err)
				}

				header.Typeflag = tar.TypeSymlink
				header.Linkname = filepath.ToSlash(link)

				// Break out of the case as a symlink
				// doesn't need any additional config.
				break
			}

			if !dereference {
				// Return early as the symlink has a target outside of the
				// src directory and we don't want to dereference symlinks.
				return nil
			}

			// Get the file info for the target.
			info, err = os.Lstat(target)
			if err != nil {
				return fmt.Errorf("failed to get file info from file %q: %w", target, err)
			}

			// If the target is a directory we can recurse into the target
			// directory by calling the packWalkFn with updated arguments.
			if info.IsDir() {
				return filepath.Walk(target, packWalkFn(base, target, path, tarW, meta, dereference, matcher))
			}

			// Dereference this symlink by updating the header with the target file
			// details and set writeBody to true so the body will be written.
			header.Typeflag = tar.TypeReg
			header.ModTime = info.ModTime()
			header.Mode = int64(info.Mode().Perm())
			header.Size = info.Size()
			writeBody = true

		default:
			return fmt.Errorf("Unexpected file mode %v", fm)
		}

		// Write the header first to the archive.
		if err := tarW.WriteHeader(header); err != nil {
			return fmt.Errorf("failed writing archive header for file %q: %w", path, err)
		}

		// Account for the file in the list.
		meta.Files = append(meta.Files, header.Name)

		klog.V(2).Infof("adding path %s", path)

		// Skip writing file data for certain file types (above).
		if !writeBody {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed opening file %q for archiving: %w", path, err)
		}
		defer f.Close()

		size, err := io.Copy(tarW, f)
		if err != nil {
			return fmt.Errorf("failed copying file %q to archive: %w", path, err)
		}

		// Add the size we copied to the body.
		meta.Size += size

		return nil
	}
}

// Meta provides detailed information about a slug.
type Meta struct {
	// The list of files contained in the slug.
	Files []string

	// Total size of the slug in bytes.
	Size int64

	// Total size of the slug in bytes after compression.
	CompressedSize int64
}

func Unpack(r io.Reader, dst string) error {
	// Decompress as we read.
	uncompressed, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to uncompress slug: %w", err)
	}

	// Untar as we read.
	untar := tar.NewReader(uncompressed)

	// Unpackage all the contents into the directory.
	for {
		header, err := untar.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to untar archive: %w", err)
		}

		// Get rid of absolute paths.
		path := header.Name
		if path[0] == '/' {
			path = path[1:]
		}
		path = filepath.Join(dst, path)

		klog.V(1).Infof("extracting %s to %s\n", header.Name, path)

		// Make the directories to the path.
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", dir, err)
		}

		// If we have a symlink, just link it.
		if header.Typeflag == tar.TypeSymlink {
			if err := os.Symlink(header.Linkname, path); err != nil {
				return fmt.Errorf("failed creating symlink %q => %q: %w",
					path, header.Linkname, err)
			}
			continue
		}

		// Only unpack regular files from this point on.
		if header.Typeflag == tar.TypeDir {
			continue
		} else if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("failed creating %q: unsupported type %c", path,
				header.Typeflag)
		}

		// Open a handle to the destination.
		fh, err := os.Create(path)
		if err != nil {
			// This mimics tar's behavior wrt the tar file containing duplicate files
			// and it allowing later ones to clobber earlier ones even if the file
			// has perms that don't allow overwriting.
			if os.IsPermission(err) {
				os.Chmod(path, 0600)
				fh, err = os.Create(path)
			}

			if err != nil {
				return fmt.Errorf("failed creating file %q: %w", path, err)
			}
		}

		// Copy the contents.
		_, err = io.Copy(fh, untar)
		fh.Close()
		if err != nil {
			return fmt.Errorf("failed to copy slug file %q: %w", path, err)
		}

		// Restore the file mode. We have to do this after writing the file,
		// since it is possible we have a read-only mode.
		mode := header.FileInfo().Mode()
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("failed setting permissions on %q: %w", path, err)
		}
	}
	return nil
}

// Construct a config map resource containing an archive. The archive is built
// from the root module at path, within the git repo base.
func ConfigMap(namespace, name, path, base string) (*corev1.ConfigMap, error) {
	// Construct new archive
	arc, err := NewArchive(path, base)
	if err != nil {
		return nil, err
	}

	// Add local module references to archive
	if err := arc.Walk(); err != nil {
		return nil, err
	}

	w := new(bytes.Buffer)
	meta, err := arc.Pack(w)
	if err != nil {
		return nil, err
	}

	klog.V(1).Infof("slug created: %d files; %d (%d) bytes (compressed)\n", len(meta.Files), meta.Size, meta.CompressedSize)

	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		BinaryData: map[string][]byte{
			v1alpha1.RunDefaultConfigMapKey: w.Bytes(),
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(&configMap)
	// Permit filtering etok resources by component
	labels.SetLabel(&configMap, labels.RunComponent)

	return &configMap, nil
}

// checkFileMode is used to examine an os.FileMode and determine if it should
// be included in the archive, and if it has a data body which needs writing.
func checkFileMode(m os.FileMode) (keep, body bool) {
	switch {
	case m.IsDir():
		return true, false

	case m.IsRegular():
		return true, true

	case m&os.ModeSymlink != 0:
		return true, false
	}

	return false, false
}
