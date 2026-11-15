# Changelog

## 0.1.0

- First Vibe preview: a terminal-first workspace panel, Explorer and Source
  Control on the right, and an editor for files and diffs.
- Each workspace starts its own shell at launch; closing the last editor tab
  returns to the terminal.
- Local terminals use `creack/pty` on macOS and Linux, which fixes shells
  exiting with code 129.
- The workspace panel can be resized and remembers its width.
- Removed the AI commit button, sessions, GitHub views, remote SSH, plugins
  and account menus from this build.
- Vibe sends no usage counts, crash reports or update checks.
