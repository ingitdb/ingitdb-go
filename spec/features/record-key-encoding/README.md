---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Record key encoding

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-key-encoding?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-key-encoding?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-key-encoding?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-key-encoding?op=request-change) |
**Status:** Draft
**Date:** 2026-11-14
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

A collection may declare `record_file.key_encoding: tilde-v1`. That names one reversible algorithm that turns a record key into a file or directory name and back. Every reader and writer then uses it, and every reader shows the decoded key. A definition that declares no encoding keeps today's behaviour: the name is the key as it is.

## Problem

inGitDB takes a record's key from its file name. That works while keys are safe file names. A database mirrored from Firestore has document ids that are not safe:

- `/`, `\` and `:`;
- the ids `.` and `..`;
- a leading dot, which makes the file hidden from inGitDB;
- a trailing dot or space, which Windows drops;
- `CON` and the other Windows device names;
- non-ASCII text that macOS may re-normalise;
- ids up to 1,500 bytes long.

The DataTug mirror invented its own escaping (`~2E`) to write such ids. inGitDB then showed the escaped name as the key, because it had no way to decode it, and foreign keys holding the real id did not match.

## Behavior

### REQ: key-encoding-declared

`record_file.key_encoding` names the algorithm.

- Allowed values: `none` and `tilde-v1`.
- Omitted or `none`: the name is the key as it is, exactly as before this feature.
- Any other value is a definition error.

The setting governs the collection's record keys wherever a key becomes a name:

- the `{key}` part of `record_file.name`;
- the directory, below the records directory, that holds a record's subcollection instances (see `subcollection-record-validation#req:subcollection-storage-convention`).

A subcollection declares its own `key_encoding` for its own keys. Keys inside a shared records file (`map[$record_id]map[$field_name]any`) are not names and are not encoded there. Their subcollection directories are encoded.

### REQ: tilde-v1-algorithm

`tilde-v1` maps a key, a non-empty string of bytes, to a name, one byte at a time:

- **Kept as they are:** the bytes `A`–`Z`, `a`–`z`, `0`–`9`, and `-` `_` `.` `@` `+` `=` `,`.
- **Written as an escape:** every other byte is written as `~` followed by its value in two upper-case hexadecimal digits. This includes `~` (`~7E`), `/` (`~2F`), `\`, `:`, space, `$`, `%`, control bytes, and every byte from `0x80` up.
- **Edge dots:** a `.` that is the first or the last byte is written as `~2E`.
- **Windows device names:** the first byte is written as an escape when the key, up to its first `.` and in any letter case, is `CON`, `PRN`, `AUX`, `NUL`, or `COM0`–`COM9` or `LPT0`–`LPT9`.
- **Empty key:** an empty key has no name.

So a name is pure ASCII. It is never hidden, never `.` or `..`, never ends in a dot or a space, never contains a character Windows, macOS or Linux reserves, and never collides with inGitDB's own `$records`, `$views` or `$ingitdb`.

| Key | Name |
|---|---|
| `abc`, `Firestore-Auto_Id09` | unchanged |
| `a/b` | `a~2Fb` |
| `..` | `~2E~2E` |
| `.hidden` | `~2Ehidden` |
| `trailing.` | `trailing~2E` |
| `a b` | `a~20b` |
| `k3v9q:agr-swim-2026` | `k3v9q~3Aagr-swim-2026` |
| `$records` | `~24records` |
| `CON` | `~43ON` |
| `con.txt` | `~63on.txt` |
| `é` (NFC, U+00E9) | `~C3~A9` |
| `é` (NFD, `e` + U+0301) | `e~CC~81` |

**Decoding** reverses each escape. A name decodes only if encoding the result gives the same name back, so each key has exactly one name. The following names belong to no key, and a reader reports them as errors:

- a lower-case escape (`~2f`);
- an incomplete escape (`~2`);
- an escape of a kept byte (`~41`);
- a byte that should have been escaped (a space, a leading dot);
- an empty name.

`ingitdb.EncodeRecordKey(encoding, key)` and `ingitdb.DecodeRecordKey(encoding, name)` in `github.com/ingitdb/ingitdb-go/ingitdb` implement this. `RecordFileDef.EncodeKey` and `RecordFileDef.DecodeKey` call them with the collection's setting. Every Go reader and writer in ingitdb-go, dalgo2ingitdb and dalgo2ingitdb4local calls these functions. The DataTug mirror has its own copy of the scheme, which it names `tilde-hex/v1`; it is byte-identical to `tilde-v1` (checked on 300,000 random ids and the mirror's documented examples, on 2026-11-14), and the mirror should call these functions and record the name `tilde-v1`. Other implementations, such as the TypeScript client, follow this text.

### REQ: letter-case-kept-collisions-reported

`tilde-v1` keeps letter case: Firestore ids are case-sensitive, and most generated ids mix cases. On a case-insensitive file system (Windows, macOS by default), two names that differ only in case are one file. So:

- Validation MUST report an error for each group of records of one collection instance whose key names differ only in letter case, whether the name is a file (`{key}.yaml`) or a directory (`{key}/index.yaml`), and for each group of subcollection parent directories that differ only in letter case.
- A writer MUST refuse to create a record when its file name, or a directory on its path, equals an existing entry's name when letter case is ignored, unless it is the same name.
- Reads and deletes are not checked. On a case-insensitive file system, a read or delete of `ab` reaches an existing `Ab`.

### REQ: over-long-key-fails

A collection that keeps one record per file (a `{key}` in `record_file.name`) stores a key only if every path segment the key produces fits 255 bytes: the encoded key when the collection declares `key_encoding`, the key as it is otherwise. This applies to every such collection, with or without `key_encoding`. That is the file name with its extension, and the record's subcollection directory. Every `{key}` of `record_file.name` is substituted, as the drivers do. Other placeholders are not measured.

A longer key is never hashed or truncated:

- A writer MUST fail with `ingitdb.ErrKeyNotStorable`. The error names the length and the remedy: keep the collection's records in one shared file, `record_file.type: map[$record_id]map[$field_name]any`.
- Validation reports any longer name it finds as an error.
- The GitHub driver applies the same check before it commits, so it never writes a tree that no checkout can hold.

Compatibility: for a collection without `key_encoding` this adds an error only where a write already failed (every common file system refuses a name over 255 bytes, so a local write used to fail with a bare "file name too long") or where the GitHub driver committed a name that no local checkout can create. No write that works today changes.

### REQ: hidden-files-skipped

In a collection that declares `key_encoding`, no key encodes to a name starting with `.`. A reader listing the collection's records MUST skip such a file (an editor lock, a macOS `._` file) instead of failing the whole read. Validation MUST still report it as a name the encoding never writes.

### Rollout

A tool built before this feature does not know `key_encoding`. Readers that decode definitions strictly reject such a definition (unknown field). Others ignore the field and show the encoded names as keys: the ingitdb-cli reading a database from GitHub, the TypeScript client, and the GitHub driver before it refuses the setting. Upgrade every reader of a database before declaring the setting in it.

### Rationale

`tilde-v1` is the DataTug mirror's scheme, unchanged. Its names stay valid, and no inGitDB database on the reference machine had a `~` in a record name (518 roots scanned on 2026-11-14), so declaring it changes no existing key.

URL percent-encoding was considered. It needs the same extensions for edge dots, device names and case, and its query form turns `+` into a space. It also clashes with the `%2F` that `dalgo2ingitdb` already writes for `/` in collections without an encoding.

## Acceptance Criteria

### AC: tilde-v1-examples

**Requirements:** record-key-encoding#req:tilde-v1-algorithm

**Given** the keys in the table above, and control, reserved and non-UTF-8 bytes
**When** they are encoded and decoded with `tilde-v1`
**Then** each gives the name shown, and decodes back to the key

### AC: tilde-v1-one-name-per-key

**Requirements:** record-key-encoding#req:tilde-v1-algorithm

**Given** the names `a~2fb`, `a~2`, `~`, `a~41`, `a b`, `.x`, `x.`, `CON` and the empty name
**When** they are decoded with `tilde-v1`
**Then** each fails with `ErrInvalidKeyName`, naming the name the encoding would write

### AC: tilde-v1-round-trip

**Requirements:** record-key-encoding#req:tilde-v1-algorithm

**Given** 20,000 random keys of hostile bytes, and the fuzz corpus
**When** each is encoded and decoded
**Then** it decodes back to itself, no two keys share a name, and every name is a safe file name (kept bytes and `~` only; not hidden, not a dot alias, no trailing dot, not a device name)

### AC: readers-use-decoded-keys

**Requirements:** record-key-encoding#req:key-encoding-declared

**Given** a `tilde-v1` collection with files `~43ON.yaml` and `a~2Fb.yaml`, a subcollection instance below `a~2Fb/`, and foreign keys holding `a/b`, `CON` and `a~2Fb`
**When** the database is validated, or a view reads the records
**Then** the keys are `CON` and `a/b`, the instance's parent key is `a/b`, the foreign keys `a/b` and `CON` match, and `a~2Fb` does not

### AC: invalid-names-and-collisions-reported

**Requirements:** record-key-encoding#req:tilde-v1-algorithm, record-key-encoding#req:letter-case-kept-collisions-reported

**Given** in a `tilde-v1` collection the record files `bad~2f.yaml`, `Ab.yaml` and `ab.yaml`, and a subcollection directory below `x~zz/`
**When** the database is validated
**Then** there is one error naming `bad~2f.yaml` and the name of its key (`bad~2F`), one naming `Ab.yaml, ab.yaml`, and one naming the `x~zz` instance directory

### AC: over-long-key-fails

**Requirements:** record-key-encoding#req:over-long-key-fails

**Given** a `tilde-v1` collection with `name: "{key}.yaml"` and a key whose encoded file name is 263 bytes
**When** the record file name is computed, or a found name of that length is validated
**Then** it fails with `ErrKeyNotStorable` naming 263 bytes, the 255-byte limit and the shared-file remedy; a 255-byte name and a shared-file collection are not affected; a collection without an encoding fails the same way for a key over 250 bytes (`{key}.yaml` as it is)

## Open Questions

- A root-level default (`.ingitdb/settings.yaml`) for `key_encoding` was not added. A collection declares it, and a mirror tool writes it into every definition it generates.
