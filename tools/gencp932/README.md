# CP932 table generator

`gencp932` regenerates `internal/parser/cp932table.go` from Microsoft's
CP932-to-Unicode mapping table distributed by the Unicode Consortium.

- Source: <https://www.unicode.org/Public/MAPPINGS/VENDORS/MICSFT/WINDOWS/CP932.TXT>
- Table version: 2.01 (1998-04-15), Unicode 2.0
- SHA-256: `c9bc0b0cd42e0fbcb82a09635bb5abed86afbdd4abc9e76fa5716638217cb59f`
- Distribution terms: <https://www.unicode.org/copyright.html>

The source table omits the CP932 user-defined range. The generator adds the
Windows-compatible mapping from `0xF040..0xF9FC` to `U+E000..U+E757` as
specified by the Shift_JIS decoder in the
[Encoding Standard](https://encoding.spec.whatwg.org/#shift_jis-decoder).

From the repository root, regenerate and inspect the result with:

```sh
go generate ./internal/parser
git diff -- internal/parser/cp932table.go
```

The generator downloads the source over HTTPS and rejects it unless the
checksum matches. For an offline or separately audited copy, place the file
under the ignored `tmp/` directory and run:

```sh
go run ./tools/gencp932 \
  -input tmp/CP932.TXT \
  -output internal/parser/cp932table.go
```
