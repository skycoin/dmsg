'use strict'

const fs = require('fs');

console.log('Starting to inject the dependencies.', '\n');

// Load the HTML to edit.
let termHtmlLocation = '../term.html';
if (!fs.existsSync(termHtmlLocation)) {
  exitWithError('ERROR: Unable to find the term HTML file. No changes were made.');
}
let currentData = fs.readFileSync(termHtmlLocation, 'utf8');

// Add the xterm CSS.
let cssLocation = './node_modules/xterm/css/xterm.css';
if (!fs.existsSync(cssLocation)) {
  exitWithError('ERROR: Unable to find the xterm CSS file. No changes were made.');
}
let cssData = fs.readFileSync(cssLocation, 'utf8');
currentData = replaceContent(currentData, cssData, '/* term-css-start */', '/* term-css-end */');

// Add the xterm JS.
let xtermLocation = './node_modules/xterm/lib/xterm.js';
if (!fs.existsSync(xtermLocation)) {
  exitWithError('ERROR: Unable to find the xterm JS file. No changes were made.');
}
let xtermData = fs.readFileSync(xtermLocation, 'utf8');
currentData = replaceContent(currentData, xtermData, '/* term-js-start */', '/* term-js-end */');

// Add the attach addon.
let attachLocation = './node_modules/xterm-addon-attach/lib/xterm-addon-attach.js';
if (!fs.existsSync(attachLocation)) {
  exitWithError('ERROR: Unable to find the xterm attach addon file. No changes were made.');
}
let attachData = fs.readFileSync(attachLocation, 'utf8');
currentData = replaceContent(currentData, attachData, '/* term-attach-start */', '/* term-attach-end */');

// Add the fit addon.
let fitLocation = './node_modules/xterm-addon-fit/lib/xterm-addon-fit.js';
if (!fs.existsSync(fitLocation)) {
  exitWithError('ERROR: Unable to find the xterm fit addon file. No changes were made.');
}
let fithData = fs.readFileSync(fitLocation, 'utf8');
currentData = replaceContent(currentData, fithData, '/* term-fit-start */', '/* term-fit-end */');

// Save the new file.
fs.writeFileSync('../term.html', currentData, {encoding: 'utf8'});
console.log('Dependencies injected.', '\n');

/**
 * Takes the text of the newData params and adds it to the currentData string, replacing everything
 * between the startString and endString params.
*/
function replaceContent(currentData, newData, startString, endString) {
  let startIndex = currentData.indexOf(startString) + (startString).length;
  let endIndex = currentData.indexOf(endString);

  return currentData.substring(0, startIndex) + '\n' + newData + '\n    ' + currentData.substring(endIndex)
}
