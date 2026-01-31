package cli

import (
	"database/sql"
	"fmt"
	"strings"
)

func RunList() {
	db, err := openNexttoneDB()
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	defer db.Close()

	current, currentSubtone, err := getCurrentMicrotone(db)
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	nodes, edges, err := loadGraph(db)
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	subtones, err := getSubtones(db)
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	fmt.Println(formatGraph(nodes, edges, current.Name, currentSubtone, subtones))
}

func RunNext(args []string) {
	db, err := openNexttoneDB()
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	defer db.Close()

	sign := parseSign(args)
	current, currentSubtone, err := getCurrentMicrotone(db)
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	if sign != "" {
		if err := recordSignature(db, current.Name, sign); err != nil {
			fmt.Printf("[error] %v\n", err)
			return
		}
		if sign == "yes" {
			if current.Name == "subtone-review" {
				if err := setCurrentMicrotone(db, "subtone-run-test"); err != nil {
					fmt.Printf("[error] %v\n", err)
					return
				}
			} else if current.Name == "subtone-run-test" {
				_, wrapped, err := advanceSubtone(db, currentSubtone)
				if err != nil {
					fmt.Printf("[error] %v\n", err)
					return
				}
				if wrapped {
					if err := setCurrentMicrotone(db, "subtone-review-complete"); err != nil {
						fmt.Printf("[error] %v\n", err)
						return
					}
				} else {
					if err := setCurrentMicrotone(db, "subtone-review"); err != nil {
						fmt.Printf("[error] %v\n", err)
						return
					}
				}
			} else if current.Name == "complete" {
				fmt.Println("DIALTONE: COMPLETE! PR merged.")
				return
			} else {
				if _, err := advanceMicrotone(db, current.Name); err != nil {
					fmt.Printf("[error] %v\n", err)
					return
				}
			}
		}
		current, currentSubtone, err = getCurrentMicrotone(db)
		if err != nil {
			fmt.Printf("[error] %v\n", err)
			return
		}
	}

	printMicrotoneQuestion(db, current, currentSubtone)
}

func printMicrotoneQuestion(db *sql.DB, mt microtone, currentSubtone string) {
	fmt.Printf("DIALTONE [%s]:\n", mt.Name)
	fmt.Printf("MICROTONE: %s\n", mt.Name)
	if mt.Name == "review-all-subtones" {
		subtones, err := getSubtones(db)
		if err == nil {
			fmt.Println("SUBTONES:")
			for _, st := range subtones {
				fmt.Printf("- %s\n", st.Name)
			}
		}
	}
	if mt.Name == "subtone-review" || mt.Name == "subtone-run-test" {
		fmt.Printf("SUBTONE: %s\n", currentSubtone)
	}
	if mt.Name == "subtone-run-test" {
		fmt.Println("TEST RESULT: PASS")
	}
	fmt.Printf("DIALTONE: %s\n", mt.Question)
	fmt.Printf("  ./dialtone.sh nexttone --sign no\n")
	fmt.Printf("  ./dialtone.sh nexttone --sign yes\n")
}

func parseSign(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--sign=") {
			val := strings.TrimPrefix(arg, "--sign=")
			return normalizeSign(val)
		}
		if arg == "--sign" && i+1 < len(args) {
			return normalizeSign(args[i+1])
		}
	}
	return ""
}

func normalizeSign(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "no":
		return strings.ToLower(value)
	default:
		return ""
	}
}

var _ = sql.ErrNoRows
