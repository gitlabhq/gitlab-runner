# zipextra

[![](https://godoc.org/github.com/saracen/zipextra?status.svg)](http://godoc.org/github.com/saracen/zipextra)

`zipextra` is a library for encoding and decoding ZIP archive format's
"Extra Fields".

The intention is to eventually support and provide a low-level API for the
majority of PKWARE's and Info-ZIP's extra fields.

Contributions are welcome.

## Supported Fields

| Identifier | Name                       |
| ---------- | -------------------------- |
| `0x000a`   | NTFS                       |
| `0x6375`   | Info-ZIP's Unicode Comment |
| `0x7875`   | Info-ZIP's New Unix        |

### Example

```
func ExampleZipExtra() {
	// create temporary file
	w, err := ioutil.TempFile("", "zipextra-example")
	if err != nil {
		panic(err)
	}

	// create new zip writer
	zw := zip.NewWriter(w)

	// create a new zip file header
	fh := &zip.FileHeader{Name: "test_file.txt"}

	// add some extra fields
	fh.Extra = append(fh.Extra, zipextra.NewInfoZIPNewUnix(big.NewInt(1000), big.NewInt(1000)).Encode()...)
	fh.Extra = append(fh.Extra, zipextra.NewInfoZIPUnicodeComment("Hello, 世界").Encode()...)

	// create the file
	fw, err := zw.CreateHeader(fh)
	if err != nil {
		panic(err)
	}
	fw.Write([]byte("foobar"))
	zw.Close()

	// open the newly created zip
	zr, err := zip.OpenReader(w.Name())
	if err != nil {
		panic(err)
	}
	defer zr.Close()

	// parse extra fields
	fields, err := zipextra.Parse(zr.File[0].Extra)
	if err != nil {
		panic(err)
	}

	// print extra field information
	for id, field := range fields {
		switch id {
		case zipextra.ExtraFieldUnixN:
			unix, _ := field.InfoZIPNewUnix()
			fmt.Printf("UID: %d, GID: %d\n", unix.Uid, unix.Gid)

		case zipextra.ExtraFieldUCom:
			ucom, _ := field.InfoZIPUnicodeComment()
			fmt.Printf("Comment: %s\n", ucom.Comment)
		}
	}
	// Output:
	// UID: 1000, GID: 1000
	// Comment: Hello, 世界
}
```