package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pb_file>")
		os.Exit(1)
	}
	fmt.Printf("DEBUG: os.Args: %v\n", os.Args)
	fmt.Printf("DEBUG: Reading file: %s\n", os.Args[1])

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	// Try extracting strings from the whole file first (declarative dump)
	// But if it looks like length-prefixed, we iterate.
	// Actually, let's just dump recursively from the top. 
	// If it's length prefixed, the first "tag" will be invalid field number or wire type.
	// But let's try to parse as sequence of length-prefixed messages manually.
	
	/*
	offset := 0
	for offset < len(data) {
		// Read varint length
		v, n := protowire.ConsumeVarint(data[offset:])
		if n < 0 {
			// Not a valid varint, maybe just raw proto?
			// Fallback to dumping as single text
			fmt.Println("Fallback to raw dump from offset", offset)
			dump(data[offset:], "")
			return
		}
		
		// It looked like a varint, but is it a reasonable length?
		length := int(v)
		if length > len(data[offset+n:]) || length < 0 {
			// Invalid length, probably not delimited.
			fmt.Println("Invalid length prefix, dumping raw")
			dump(data[offset:], "")
			return
		}

		// It seems to be a message of 'length' bytes
		// Dump content
		// fmt.Printf("--- Message (Len %d) ---\n", length)
		dump(data[offset+n:offset+n+length], "")
		
		offset += n + length
	}
	*/
	dump(data, "")
}

func dump(data []byte, path string) {
	fmt.Printf("DEBUG: dump called with %d bytes, path %s\n", len(data), path)
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		fmt.Printf("DEBUG: ConsumeTag: num=%d, typ=%d, n=%d\n", num, typ, n)
		if n < 0 {
			break
		}
		data = data[n:]

		if typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(data)
			if n < 0 {
				break
			}
			
			// Check recursively first
			if len(v) > 0 {
				dump(v, fmt.Sprintf("%s.%d", path, num))
			}

			if utf8.Valid(v) {
				s := string(v)
				if strings.Contains(s, "test") {
					fmt.Printf("FOUND AT PATH %s Field %d: %q\n", path, num, s)
				}
				if len(s) < 50 {
					fmt.Printf("PATH %s Field %d Val: %q\n", path, num, s)
				}
			}

			data = data[n:]
		} else if typ == protowire.VarintType {
			v, n := protowire.ConsumeVarint(data)
			if n < 0 { break }
			fmt.Printf("PATH %s Field %d (Varint): %d\n", path, num, v)
			data = data[n:]
		} else {
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 { break }
			fmt.Printf("PATH %s Field %d (Type %d)\n", path, num, typ)
			data = data[n:]
		}
	}
}
