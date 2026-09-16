// Package folder holds the state Lynx keeps for one shared folder.
package folder

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thyfriendlyfox/lynx/internal/atomicfile"
	"github.com/thyfriendlyfox/lynx/internal/sharecode"
)

// StateDir is the folder Lynx keeps inside a shared folder.
const StateDir = ".lynx"

const (
	configName = "config.json"
	indexName  = "index.json"
	keyName    = "peer.key"
	logName    = "lynx.log"
	runName    = "daemon.json"
)

// ErrNotShared reports a folder that Lynx does not manage.
var ErrNotShared = errors.New("this folder is not shared by lynx")

// ErrAlreadyShared reports a folder that is already shared.
var ErrAlreadyShared = errors.New("this folder is already shared by lynx")

// Config is the file a person can read and edit.
type Config struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	PeerName string `json:"peer_name"`
	Created  int64  `json:"created"`
	Port     int    `json:"port"`
}

// Folder is an open shared folder.
type Folder struct {
	Root   string
	Config Config
	key    []byte
}

// Init makes a new shared folder at root.
func Init(root string) (*Folder, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, configName)); err == nil {
		return nil, ErrAlreadyShared
	}
	key, err := sharecode.NewKey()
	if err != nil {
		return nil, err
	}
	return create(root, key)
}

// Join makes a shared folder at root from a key that a friend sent.
func Join(root string, key []byte) (*Folder, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if len(key) != sharecode.KeyLen {
		return nil, sharecode.ErrBadCode
	}
	if f, err := Open(root); err == nil {
		if string(f.key) == string(key) {
			return f, nil
		}
		return nil, ErrAlreadyShared
	}
	return create(root, key)
}

func create(root string, key []byte) (*Folder, error) {
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		return nil, err
	}
	f := &Folder{
		Root: root,
		key:  key,
		Config: Config{
			Version:  1,
			Name:     filepath.Base(root),
			Key:      base64.StdEncoding.EncodeToString(key),
			PeerName: DefaultPeerName(),
			Created:  time.Now().UnixMilli(),
		},
	}
	if err := f.SaveConfig(); err != nil {
		return nil, err
	}
	if err := writeStateNotice(root); err != nil {
		return nil, err
	}
	return f, nil
}

// Open reads the shared folder at root.
func Open(root string) (*Folder, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, StateDir, configName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotShared
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(StateDir, configName), err)
	}
	key, err := base64.StdEncoding.DecodeString(c.Key)
	if err != nil || len(key) != sharecode.KeyLen {
		return nil, fmt.Errorf("the folder key in %s is damaged", configName)
	}
	if c.PeerName == "" {
		c.PeerName = DefaultPeerName()
	}
	return &Folder{Root: root, Config: c, key: key}, nil
}

// FindUp looks for a shared folder at dir or above it.
func FindUp(dir string) (*Folder, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		if f, err := Open(dir); err == nil {
			return f, nil
		} else if !errors.Is(err, ErrNotShared) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNotShared
		}
		dir = parent
	}
}

// SaveConfig writes the config file.
func (f *Folder) SaveConfig() error {
	data, err := json.MarshalIndent(f.Config, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(f.ConfigPath(), append(data, '\n'), 0o600)
}

// Key returns the folder key. Anyone with this key can read and write the
// folder.
func (f *Folder) Key() []byte { return f.key }

// ShareCode returns the string to send to a friend.
func (f *Folder) ShareCode() string { return sharecode.Encode(f.key) }

// ID returns the public name of the folder on the network. It comes from the
// key by one way, so it never exposes the key.
func (f *Folder) ID() string {
	sum := sha256.Sum256(append([]byte("lynx-folder-id:"), f.key...))
	return hex.EncodeToString(sum[:8])
}

// Paths inside the state folder.
func (f *Folder) StatePath() string  { return filepath.Join(f.Root, StateDir) }
func (f *Folder) ConfigPath() string { return filepath.Join(f.StatePath(), configName) }
func (f *Folder) IndexPath() string  { return filepath.Join(f.StatePath(), indexName) }
func (f *Folder) KeyPath() string    { return filepath.Join(f.StatePath(), keyName) }
func (f *Folder) LogPath() string    { return filepath.Join(f.StatePath(), logName) }
func (f *Folder) RunPath() string    { return filepath.Join(f.StatePath(), runName) }

// DefaultPeerName returns the name this machine shows to other peers.
func DefaultPeerName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "peer"
	}
	host = strings.TrimSuffix(host, ".local")
	return host
}

func writeStateNotice(root string) error {
	notice := "This folder holds the state that Lynx keeps for the shared folder.\n" +
		"config.json holds the folder key. Anyone with that key can read and\n" +
		"write this folder. Do not copy this folder into a public place.\n"
	return atomicfile.Write(filepath.Join(root, StateDir, "README.txt"), []byte(notice), 0o644)
}
