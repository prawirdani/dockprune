# Generates assets/demo.gif. Needs vhs (https://github.com/charmbracelet/vhs)
# and the demo-only fake Docker API. Run it with `make demo`, which starts the
# fake server and points DOCKER_HOST at it — no real Docker daemon is touched.
Output assets/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 680
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

Type "./dist/dockprune"
Enter
Sleep 1.2s

# Containers: multi-select web (running), db, cache.
Space
Sleep 300ms
Down
Space
Sleep 300ms
Down
Space
Sleep 700ms

# Images tab: browse, dependency warning shows in the details pane.
Type "2"
Sleep 1.2s
Down
Sleep 700ms

# Volumes tab.
Type "3"
Sleep 1.2s
Down
Sleep 700ms

# Back to containers (selection persisted across tabs).
Type "1"
Sleep 800ms

# Live search.
Type "/"
Type "red"
Sleep 900ms
Escape
Sleep 500ms

# Confirm (dependency warnings listed), then bulk delete with live progress.
Enter
Sleep 1.5s
Type "y"
Sleep 3s
