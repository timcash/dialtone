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

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	// Try extracting strings from the whole file first (declarative dump)
	// But if it looks like length-prefixed, we iterate.
	// Actually, let's just dump recursively from the top. 
	// If it's length prefixed, the first "tag" will be invalid field number or wire type.
	// But let's try to parse as sequence of length-prefixed messages manually.
	
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
}

func dump(data []byte, path string) {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
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

			// Also check if this specific byte slice is the string
			if utf8.Valid(v) {
				s := string(v)
				// Look for part of the user's prompt
				if strings.Contains(s, "missing chat logs") {
					fmt.Printf("FOUND AT PATH %s Field %d: %q\n", path, num, s)
				}
				// Also print short strings to see roles
				if len(s) < 20 {
					fmt.Printf("PATH %s Field %d Val: %q\n", path, num, s)
				}
			}

			data = data[n:]
		} else if typ == protowire.VarintType {
			_, n := protowire.ConsumeVarint(data)
			if n < 0 { break }
			data = data[n:]
		} else {
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 { break }
			data = data[n:]
		}
	}
}
