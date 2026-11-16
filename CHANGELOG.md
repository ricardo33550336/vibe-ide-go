# Changelog

## Unreleased

- **File → Open Folder in WSL…** opens a folder of a WSL distribution, read and written by `wsl.exe` inside it.
- Explorer, quick open, search, Git and blame of a WSL folder run in its distribution, not over the `\\wsl.localhost` share.
- Terminals of a WSL folder are shells of its distribution, started in that folder.
- **Copy Path** of a file in a WSL folder copies the Linux path a shell there takes, as `/home/me/proj/main.go`.
- A folder opened from `\\wsl.localhost\Distro\…` opens in its distribution instead of through the share.
- The status bar names the distribution, with a menu to reconnect, disconnect and open a folder of it.

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
