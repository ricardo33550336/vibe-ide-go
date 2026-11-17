# Changelog

## 0.2.5

- Notes with many formulas open faster: **Live Preview** typesets them several at a time.
- The **branch switcher** draws only the rows in view, so typing into thousands of branches stays smooth.
- **Source Control** history keeps its branch and tag badges without rebuilding them every frame.
- **Source Control** no longer re-encodes every diff line each frame, so an idle window stays light.

## 0.2.4

- **Source Control** lists only the repositories with changes, so a folder of many does not bury them.
- A **Search** button beside the Explorer and Source Control switches the sidebar to search.

## 0.2.3

- **Reveal in File Explorer** shows a file or folder of a WSL folder, through the share Windows mounts it on.
- A path a program prints in a terminal of a WSL or SSH folder opens in the editor on Ctrl+click.

## 0.2.2

- A file made in a WSL folder stays open and listed, rather than showing as deleted from the disk.
- The Explorer of a WSL folder lists a file made in one of its subfolders again.

## 0.2.1

- A workspace opens without starting a shell of its own; the terminal button starts the first when you ask for one.
- **Open Folder in WSL…** connects to distributions whose `sh` is dash, as Ubuntu's, which the script's input starved.
- Commands of a WSL or SSH folder no longer flash a console window on Windows.

## 0.2.0

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
