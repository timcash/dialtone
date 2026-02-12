# Template Plugin v3 Test Report

**Generated at:** Thu, 12 Feb 2026 12:14:50 PST
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS

## 1. Preflight: Go + UI Checks

### Template Install: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh template install src_v3
[elapsed] 653ms
```

### Template Fmt: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh template fmt src_v3
[elapsed] 870ms
```

### Template Lint: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh template lint src_v3
[elapsed] 1.859s
```

### Template Build: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh template build src_v3
[elapsed] 2.135s
```

---

## 2. Expected Errors (Proof of Life)

| Level | Message | Status |
|---|---|---|
| error | "[PROOFOFLIFE] Intentional Browser Test Error" | ✅ CAPTURED |
| error | "[PROOFOFLIFE] Intentional Go Test Error" | ✅ CAPTURED |

## 3. UI & Interactivity

### 1. Hero Section Validation: PASS ✅

**Console Logs:**
```text
[log] "\"[SectionManager] INITIAL LOAD #hero\""
[log] "\"[SectionManager] NAVIGATING TO #hero\""
[log] "\"[SectionManager] LOADING #hero\""
[log] "\"[SectionManager] LOADED #hero\""
[log] "\"[SectionManager] START #hero\""
[log] "\"[SectionManager] NAVIGATE TO #hero\""
[log] "\"[SectionManager] RESUME #hero\""
[error] "\"[PROOFOFLIFE] Intentional Browser Test Error\""
```

**Assertions:**
- [OK] aria-label="Hero Section" visible
- [OK] aria-label="Hero Canvas" present

![Hero Section Validation](test_step_1.png)

## 4. Artifacts

- `test.log`
- `error.log`
- `test_step_1.png`
