# Changelog

All notable changes to Lynx are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/) · Versioning: [SemVer](https://semver.org/).

## [Unreleased]
### Added
- `lynx share` turns a folder into a shared folder and prints the code to send.
- `lynx join <code>` joins a folder that a friend shared.
- Live sync between peers: adds, edits, deletes and renames travel in about 1 second.
- Conflict copies when 2 people change the same file at the same time.
- `.lynx/trash/` keeps every file that Lynx replaces or deletes.
- `.lynxignore` keeps files local.
- `lynx status`, `lynx log`, `lynx code`, `lynx start`, `lynx stop`, `lynx run`.
- `lynx menu install` adds "Share with Lynx" to the right click menu on macOS,
  Windows and Linux.
- Peer discovery on the local network with mDNS and on the internet with the
  public libp2p distributed hash table.
- A `discovery` field in the folder config that selects `both`, `mdns` or
  `dht`.

### Changed
### Deprecated
### Removed
### Fixed
### Security
- Peers prove that they hold the folder key with an HMAC challenge before any
  file moves.
- A file path from a peer that points outside the folder is refused.
