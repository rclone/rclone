SublimeEvernote
===============

[Sublime Text 2](http://www.sublimetext.com/2) plugin for [Evernote](http://www.evernote.com) 


### Install

Through [Package Control](http://wbond.net/sublime_packages/package_control)

`Command Palette` > `Package Control: Install Package` > `SublimeEvernote`

or

`Command Palette` > `Package Control: add Repository` && `input 'http://github.com/jamiesun/SublimeEvernote`

`Command Palette` > `Package Control: Install Package` > `SublimeEvernote`

or clone this repository in

* Windows: `%APPDATA%/Roaming/Sublime Text 2/Packages/`
* OSX: `~/Library/Application Support/Sublime Text 2/Packages/`
* Linux: `~/.Sublime Text 2/Packages/`
* Portable Installation: `Sublime Text 2/Data/`

### Usage

`Command Palette` > `Send to evernote`

`Context menu` > `Send to Evernote`

`Context menu` > `Evernote settings`

#### Markdown Support ####

Write notes in Markdown and they will be processed when they are sent to Evernote.

This:
![this](https://dl.dropbox.com/u/643062/SublimeEvernoteScreenshots/Markdown.png)

Turns into this:
![this](https://dl.dropbox.com/u/643062/SublimeEvernoteScreenshots/Evernote.png)

#### Authenticating with Evernote ####

In order to send notes you need to authenticate and allow the plugin permissions via Evernote's oauth. 
This is a bit of a manual process now as there are no callbacks to Sublime to handle this process automatically.
Here are a collection of screenshots to step you through the process.

##### Step 1 - Sublime text2 open your browser,you need login:
![login](https://raw.githubusercontent.com/jamiesun/SublimeEvernote/master/snapshot/login.png)

##### Step 2 - Authorize plugin with Evernote:
![authorize](https://raw.github.com/dencold/static/master/images/sublimeevernote/2_authorize.png)

##### Step 3 - Copy oauth verifier
![redirect](https://raw.github.com/dencold/static/master/images/sublimeevernote/3_redirect.png)
![verifier](https://raw.github.com/dencold/static/master/images/sublimeevernote/4_oauth_verifier.png)

##### Step 4 - Verify token on Sublime
![redirect](https://raw.github.com/dencold/static/master/images/sublimeevernote/5_verify_sublime.png)

##### Step 5 - Rejoice!
![redirect](https://raw.github.com/dencold/static/master/images/sublimeevernote/6_rejoice.png)

#### Metadata ####

Use metadata block to specify title and tags.

    ---
    title: My Note
    tags: tag1,tag2
    ---
