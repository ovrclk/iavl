package iavl

import (
	"fmt"

	"github.com/pkg/errors"
	dbm "github.com/tendermint/tmlibs/db"
)

var ErrVersionDoesNotExist = fmt.Errorf("version does not exist")

// VersionedTree is a persistent tree which keeps track of versions.
type VersionedTree struct {
	*orphaningTree                           // The current, working tree.
	versions       map[uint64]*orphaningTree // The previous, saved versions of the tree.
	latestVersion  uint64                    // The latest saved version.
	ndb            *nodeDB
}

// NewVersionedTree returns a new tree with the specified cache size and datastore.
func NewVersionedTree(cacheSize int, db dbm.DB) *VersionedTree {
	ndb := newNodeDB(cacheSize, db)
	head := &IAVLTree{ndb: ndb}

	return &VersionedTree{
		orphaningTree: newOrphaningTree(head),
		versions:      map[uint64]*orphaningTree{},
		ndb:           ndb,
	}
}

// LatestVersion returns the latest saved version of the tree.
func (tree *VersionedTree) LatestVersion() uint64 {
	return tree.latestVersion
}

// VersionExists returns whether or not a version exists.
func (tree *VersionedTree) VersionExists(version uint64) bool {
	_, ok := tree.versions[version]
	return ok
}

// Tree returns the current working tree.
func (tree *VersionedTree) Tree() *IAVLTree {
	return tree.orphaningTree.IAVLTree
}

// String returns a string representation of the tree.
func (tree *VersionedTree) String() string {
	return tree.ndb.String()
}

// Load a versioned tree from disk. All tree versions are loaded automatically.
func (tree *VersionedTree) Load() error {
	roots, err := tree.ndb.getRoots()
	if err != nil {
		return err
	}

	// Load all roots from the database.
	for _, root := range roots {
		t := newOrphaningTree(&IAVLTree{ndb: tree.ndb})
		t.Load(root)

		version := t.root.version
		tree.versions[version] = t

		if version > tree.latestVersion {
			tree.latestVersion = version
		}
	}
	// Set the working tree to a copy of the latest.
	tree.orphaningTree = tree.versions[tree.latestVersion].Clone()

	return nil
}

// GetVersioned gets the value at the specified key and version.
func (tree *VersionedTree) GetVersioned(key []byte, version uint64) (
	index int, value []byte, exists bool,
) {
	if t, ok := tree.versions[version]; ok {
		return t.Get(key)
	}
	return -1, nil, false
}

// SaveVersion saves a new tree version to disk, based on the current state of
// the tree. Multiple calls to SaveVersion with the same version are not allowed.
func (tree *VersionedTree) SaveVersion(version uint64) ([]byte, error) {
	if _, ok := tree.versions[version]; ok {
		return nil, errors.Errorf("version %d was already saved", version)
	}
	if tree.root == nil {
		return nil, ErrNilRoot
	}
	if version == 0 {
		return nil, errors.New("version must be greater than zero")
	}
	if version <= tree.latestVersion {
		return nil, errors.Errorf("version must be greater than latest (%d <= %d)",
			version, tree.latestVersion)
	}

	tree.latestVersion = version
	tree.versions[version] = tree.orphaningTree

	tree.orphaningTree.SaveVersion(version)
	tree.orphaningTree = tree.orphaningTree.Clone()

	tree.ndb.SaveRoot(tree.root, version)
	tree.ndb.Commit()

	return tree.root.hash, nil
}

// DeleteVersion deletes a tree version from disk. The version can then no
// longer be accessed.
func (tree *VersionedTree) DeleteVersion(version uint64) error {
	if version == 0 {
		return errors.New("version must be greater than 0")
	}
	if version == tree.latestVersion {
		return errors.Errorf("cannot delete latest saved version (%d)", version)
	}
	if _, ok := tree.versions[version]; !ok {
		return errors.WithStack(ErrVersionDoesNotExist)
	}

	tree.ndb.DeleteVersion(version)
	tree.ndb.Commit()

	delete(tree.versions, version)

	return nil
}

// GetVersionedWithProof gets the value under the key at the specified version
// if it exists, or returns nil.  A proof of existence or absence is returned
// alongside the value.
func (tree *VersionedTree) GetVersionedWithProof(key []byte, version uint64) ([]byte, KeyProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetWithProof(key)
	}
	return nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}

// GetVersionedRangeWithProof gets key/value pairs within the specified range
// and limit. To specify a descending range, swap the start and end keys.
//
// Returns a list of keys, a list of values and a proof.
func (tree *VersionedTree) GetVersionedRangeWithProof(startKey, endKey []byte, limit int, version uint64) ([][]byte, [][]byte, *KeyRangeProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetRangeWithProof(startKey, endKey, limit)
	}
	return nil, nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}

// GetVersionedFirstInRangeWithProof gets the first key/value pair in the
// specified range, with a proof.
func (tree *VersionedTree) GetVersionedFirstInRangeWithProof(startKey, endKey []byte, version uint64) ([]byte, []byte, *KeyFirstInRangeProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetFirstInRangeWithProof(startKey, endKey)
	}
	return nil, nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}

// GetVersionedLastInRangeWithProof gets the last key/value pair in the
// specified range, with a proof.
func (tree *VersionedTree) GetVersionedLastInRangeWithProof(startKey, endKey []byte, version uint64) ([]byte, []byte, *KeyLastInRangeProof, error) {
	if t, ok := tree.versions[version]; ok {
		return t.GetLastInRangeWithProof(startKey, endKey)
	}
	return nil, nil, nil, errors.WithStack(ErrVersionDoesNotExist)
}