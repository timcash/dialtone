on run argv
    -- Usage:
    --   osascript ghostty-send-dialtone_mod.applescript [command] [host] [user] [repoPath]
    --
    -- Default behavior sends:
    --   ssh -tt user@gold.shad-artichoke.ts.net 'cd /Users/user/dialtone && ./dialtone_mod'
    -- into the active Ghostty window.

    set cmd to "./dialtone_mod"
    if (count of argv) > 0 then
        set cmd to item 1 of argv
    end if

    set host to "gold.shad-artichoke.ts.net"
    if (count of argv) > 1 then
        set host to item 2 of argv
    end if

    set userName to "user"
    if (count of argv) > 2 then
        set userName to item 3 of argv
    end if

    set repoPath to "/Users/user/dialtone"
    if (count of argv) > 3 then
        set repoPath to item 4 of argv
    end if

    set repoCmd to "cd " & quoted form of repoPath & " && " & cmd
    set sshCmd to "ssh -tt " & userName & "@" & host & " " & quoted form of repoCmd

    tell application "Ghostty"
        activate
    end tell

    delay 0.3

    tell application "System Events"
        tell process "Ghostty"
            keystroke sshCmd
            key code 36
        end tell
    end tell
end run
