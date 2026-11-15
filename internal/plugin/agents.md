# A Vibe plugin

This folder is a plugin for [Vibe](https://github.com/sorrycc/Vibe), a code editor written in Go. Notes for
agents and people working on it.

## How it runs

- `plugin.json` declares the plugin: its `name`, its entry module `main`, when it starts (`activation`) and what it
  adds (`contributes`: commands, keybindings, menus, settings, status items, views, previews, formatters, themes,
  languages, snippets, coding agents for the Sessions view). Every id starts with the plugin's name and a dot.
- `main.ts` runs in Vibe's own JavaScript engine (moejs), not Node. TypeScript runs as it is, with no build step.
- The API is the `vibe` module; its types are in `vibe.d.ts`. Every call that touches the editor returns a promise.
- There is no `require`, no `node:` module and no npm package at run time. Import relative files of this folder. To use
  a package, run `vibe plugin pack .`, which copies what the code imports into `vendor/`.
- `fetch`, `URL`, `setTimeout` and the Streams are there; `eval` and `new Function` are not.
- Files and programs go through `workspace` and `process`, which work on SSH folders too.

## Try it

```sh
vibe plugin dev .      # load this folder into Vibe, and again on each save
vibe run <command-id>  # run one of its commands
```

Logs from `console.log` and errors show in Vibe under **Plugins: Show Log**.

## Limits

A command's JavaScript that runs for 10 seconds without a break is stopped, and so is a plugin that keeps more than
256 MiB. A plugin that stops 3 times in a row is off until **Plugins: Reload Plugins**. A file larger than 1 MiB is
not loaded.
