# Terminal code (dmsgpty-ui)

The `therm.html` file contains the code for the dmsgpty-ui terminal. It is made mainly of 2 parts:

- The first one is the code of [xterm.js](https://github.com/xtermjs/xterm.js) and its
dependencies. There are 2 big code comments indicating where this code starts and ends.
You should not modify this code, as it is overwritten by a script every time xterm.js
is updated and removing some comments could make the updater stop working.

- The seconds parts is the code for managing xterm.js, it is just after the comment indicating
the end of the xterm.js code. This is the part that can be modiffied to alter the functionality.

## Xterm.js docs

You can find documentation on the projects page: http://xtermjs.org/

## Updating Xterm.js

Xterm.js is a NPM package, so you need to have Node.js installed to be able to update it.

For updating:

- First go to the [term_deps](./term_deps) folder. There you can find the
[package.json](./term_deps/package.json) file were you can set the desired version. For
installing the newly specified version or just letting NPM update to the newest applicable
version (if the version number was not changed), run `npm install`. That will create a
`node_modules` with the updated dependencies.

- After updating the `node_modules` folder, still inside the [term_deps](./term_deps) folder,
run `node inject_deps.js`, to copy the code of the dependencies to the `therm.html` file.

### How inject_deps.js works

This script loads and injects 4 code segments to `therm.html`:

- `xterm.css`: is loaded from `./term_deps/node_modules/xterm/css/xterm.css` and added between the
`/* term-css-start */` and `/* term-css-end */` strings inside the `therm.html` file.

- `xterm.js`: is loaded from `./term_deps/node_modules/xterm/lib/xterm.js` and added between the
`/* term-js-start */` and `/* term-js-end */` strings inside the `therm.html` file.

- `xterm-addon-attach.js`: is loaded from `./term_deps/node_modules/xterm-addon-attach/lib/xterm-addon-attach.js`
and added between the `/* term-attach-start */` and `/* term-attach-end */` strings inside the `therm.html` file.

- `xterm-addon-fit.js`: is loaded from `./term_deps/node_modules/xterm-addon-fit/lib/xterm-addon-fit.js`
and added between the `/* term-fit-start */` and `/* term-fit-end */` strings inside the `therm.html` file.

As you can see, the `inject_deps.js` script uses specific start and end comments to know were the
content must be added. These comments must remain on the `therm.html` file for the updater to work
and you can use then to check were the code is added.
