# Docs

This directory tree is used to build all the different docs for
rclone.

See the `content` directory for the docs in markdown format.

Note that some of the docs are auto-generated - these should have a DO
NOT EDIT marker near the top.

Use [hugo](https://github.com/spf13/hugo) to build the website.

## Changing the layout

If you want to change the layout then the main files to edit are

- `layout/index.html` for the front page
- `chrome/*.html` for the HTML fragments
- `_default/single.md` for the default template
- `page/single.md` for the page template

Running `make serve` in a terminal give a live preview of the website
so it is easy to tweak stuff.

## What are all these files

```
├── config.json                   - hugo config file
├── content                       - docs and backend docs
│   ├── _index.md                 - the front page of rclone.org
│   ├── commands                  - auto-generated command docs - DO NOT EDIT
├── i18n
│   └── en.toml                   - hugo multilingual config
├── layouts                       - how the markdown gets converted into HTML
│   ├── 404.html                  - 404 page
│   ├── chrome                    - contains parts of the HTML page included elsewhere
│   │   ├── footer.copyright.html - copyright footer
│   │   ├── footer.html           - footer including scripts
│   │   ├── header.html           - the whole html header
│   │   ├── header.includes.html  - header includes e.g. css files
│   │   ├── menu.html             - left hand side menu
│   │   ├── meta.html             - meta tags for the header
│   │   └── navbar.html           - top navigation bar
│   ├── _default
│   │   └── single.html           - the default HTML page render
│   ├── index.html                - the index page of the whole site
│   ├── page
│   │   └── single.html           - the render of all "page" type markdown
│   ├── partials                  - bits of HTML to include into layout .html files
│   │   └── version.html          - the current version number
│   ├── rss.xml                   - template for the RSS output
│   ├── section                   - rendering for sections
│   │   └── commands.html         - rendering for /commands/index.html
│   ├── shortcodes                - shortcodes to call from markdown files
│   │   ├── cdownload.html        - download the "current" version
│   │   ├── download.html         - download a version with the partials/version.html number
│   │   ├── provider.html         - used to make provider list on the front page
│   │   └── version.html          - used to insert the current version number
│   └── sitemap.xml               - sitemap template
├── public                        - render of the website
├── README.md                     - this file
├── resources                     - don't know!
│   └── _gen
│       ├── assets
│       └── images
└── static                        - static content for the website
    ├── css
    │   ├── bootstrap.css
    │   ├── custom.css            - custom css goes here
    │   └── font-awesome.css
    ├── img                       - images used
    ├── js
    │   ├── bootstrap.js
    │   ├── custom.js             - custom javascript goes here
    │   └── jquery.js
    └── webfonts
```
