// rclone.js — replaces bootstrap.js + jquery + popper + custom.js

(function() {
    "use strict";

    // ===== Navbar collapse toggle =====
    document.addEventListener("click", function(e) {
        var toggler = e.target.closest(".navbar-toggler");
        if (!toggler) return;
        var targetId = toggler.getAttribute("data-target");
        if (!targetId) return;
        var target = document.querySelector(targetId);
        if (target) target.classList.toggle("show");
    });

    // ===== Dropdown toggle =====
    document.addEventListener("click", function(e) {
        var toggle = e.target.closest("[data-toggle='dropdown']");

        // Close all open dropdowns first
        var openMenus = document.querySelectorAll(".dropdown-menu.show");
        for (var i = 0; i < openMenus.length; i++) {
            // Don't close the menu we're about to toggle
            if (toggle && openMenus[i].parentNode.contains(toggle)) continue;
            openMenus[i].classList.remove("show");
        }

        if (!toggle) return;
        e.preventDefault();

        var menu = toggle.nextElementSibling;
        if (menu && menu.classList.contains("dropdown-menu")) {
            menu.classList.toggle("show");
        }
    });

    // ===== Mega menu: mobile headers + filter input =====
    var preScrollMenus = document.querySelectorAll(".dropdown-menu.pre-scrollable");
    for (var m = 0; m < preScrollMenus.length; m++) {
        (function(menu) {
            var toggleBtn = menu.previousElementSibling;
            if (!toggleBtn) return;
            var label = toggleBtn.textContent.trim().replace(/\s*▾$/, "");

            // Mobile close header
            var header = document.createElement("div");
            header.className = "dropdown-mobile-header";
            var labelSpan = document.createElement("span");
            labelSpan.textContent = label;
            var closeSpan = document.createElement("span");
            closeSpan.className = "dropdown-mobile-close";
            closeSpan.innerHTML = "&times;";
            header.appendChild(labelSpan);
            header.appendChild(closeSpan);
            menu.insertBefore(header, menu.firstChild);
            header.addEventListener("click", function() {
                menu.classList.remove("show");
            });

            // Filter input (desktop + mobile)
            var filterWrap = document.createElement("div");
            filterWrap.className = "dropdown-filter-wrap";
            var filterInput = document.createElement("input");
            filterInput.className = "dropdown-filter-input";
            filterInput.type = "text";
            filterInput.placeholder = "Filter\u2026";
            filterInput.setAttribute("autocomplete", "off");
            filterWrap.appendChild(filterInput);
            // Insert after mobile header
            header.insertAdjacentElement("afterend", filterWrap);

            // Prevent clicks on the filter from closing the dropdown
            filterWrap.addEventListener("click", function(e) { e.stopPropagation(); });

            // Filtering logic
            var items = menu.querySelectorAll(".dropdown-item");
            var headings = menu.querySelectorAll(".dropdown-letter-heading");
            var dividers = menu.querySelectorAll(".dropdown-divider");

            filterInput.addEventListener("input", function() {
                var query = filterInput.value.toLowerCase();

                // Show/hide items
                for (var i = 0; i < items.length; i++) {
                    if (!query || items[i].textContent.toLowerCase().indexOf(query) !== -1) {
                        items[i].classList.remove("filter-hidden");
                    } else {
                        items[i].classList.add("filter-hidden");
                    }
                }

                // Show/hide letter headings: hide if all following items until next heading/divider are hidden
                for (var h = 0; h < headings.length; h++) {
                    var hasVisible = false;
                    var sibling = headings[h].nextElementSibling;
                    while (sibling && !sibling.classList.contains("dropdown-letter-heading") && !sibling.classList.contains("dropdown-divider")) {
                        if (sibling.classList.contains("dropdown-item") && !sibling.classList.contains("filter-hidden")) {
                            hasVisible = true;
                            break;
                        }
                        sibling = sibling.nextElementSibling;
                    }
                    if (hasVisible || !query) {
                        headings[h].classList.remove("filter-hidden");
                    } else {
                        headings[h].classList.add("filter-hidden");
                    }
                }

                // Hide dividers if filtering is active
                for (var d = 0; d < dividers.length; d++) {
                    if (query) {
                        dividers[d].classList.add("filter-hidden");
                    } else {
                        dividers[d].classList.remove("filter-hidden");
                    }
                }
            });

            // Enter: if exactly one visible result, navigate to it
            filterInput.addEventListener("keydown", function(e) {
                if (e.key === "Enter") {
                    var visible = [];
                    for (var i = 0; i < items.length; i++) {
                        if (!items[i].classList.contains("filter-hidden")) visible.push(items[i]);
                    }
                    if (visible.length === 1 && visible[0].href) {
                        window.location.href = visible[0].href;
                    }
                }
            });

            // Escape: clear filter, or close dropdown if already clear
            filterInput.addEventListener("keydown", function(e) {
                if (e.key === "Escape") {
                    if (filterInput.value) {
                        filterInput.value = "";
                        filterInput.dispatchEvent(new Event("input"));
                        e.stopPropagation();
                    } else {
                        menu.classList.remove("show");
                    }
                }
            });

            // Store reference for auto-focus
            menu._filterInput = filterInput;
        })(preScrollMenus[m]);
    }

    // Auto-focus filter input when a pre-scrollable dropdown opens, clear on close
    document.addEventListener("click", function(e) {
        var toggle = e.target.closest("[data-toggle='dropdown']");
        if (!toggle) return;
        var menu = toggle.nextElementSibling;
        if (!menu || !menu.classList.contains("pre-scrollable") || !menu._filterInput) return;
        // Defer to next frame so the .show class has been toggled
        setTimeout(function() {
            if (menu.classList.contains("show")) {
                menu._filterInput.value = "";
                menu._filterInput.dispatchEvent(new Event("input"));
                menu._filterInput.focus();
            }
        }, 0);
    });

    // ===== Header hover links =====
    var headings = document.querySelectorAll("h2, h3, h4, h5, h6");
    for (var j = 0; j < headings.length; j++) {
        var el = headings[j];
        var id = el.getAttribute("id");
        if (id) {
            var a = document.createElement("a");
            a.className = "header-link";
            a.href = "#" + id;
            a.innerHTML = '<svg class="icon"><use href="#icon-link"/></svg>';
            el.insertBefore(a, el.firstChild);
        }
    }

    // ===== TOC expand / collapse =====
    var colToc = document.querySelector(".col-toc:not(.col-toc-empty)");
    var tocSidebar = colToc ? colToc.querySelector(".toc-sidebar") : null;
    if (colToc) {
        var tocToggle = colToc.querySelector(".toc-toggle");

        // Toggle button opens/closes the overlay panel
        if (tocToggle) {
            tocToggle.addEventListener("click", function(e) {
                e.stopPropagation();
                colToc.classList.toggle("toc-open");
            });
        }

        // Clicking a TOC link closes the overlay
        if (tocSidebar) {
            tocSidebar.addEventListener("click", function(e) {
                if (e.target.closest("a")) {
                    colToc.classList.remove("toc-open");
                }
            });
        }

        // Clicking outside the panel closes it
        document.addEventListener("click", function(e) {
            if (!colToc.classList.contains("toc-open")) return;
            if (e.target.closest(".toc-sidebar") || e.target.closest(".toc-toggle")) return;
            colToc.classList.remove("toc-open");
        });
    }

    // ===== TOC active section highlighting =====
    if (tocSidebar) {
        var tocLinks = tocSidebar.querySelectorAll("nav#TableOfContents a");

        // Tooltip for truncated TOC links
        var tocTooltip = document.createElement("div");
        tocTooltip.className = "toc-tooltip";
        tocSidebar.appendChild(tocTooltip);

        for (var i = 0; i < tocLinks.length; i++) {
            (function(link) {
                link.addEventListener("mouseenter", function() {
                    if (link.scrollWidth <= link.clientWidth) return;
                    tocTooltip.textContent = link.textContent;
                    var linkRect = link.getBoundingClientRect();
                    var sidebarRect = tocSidebar.getBoundingClientRect();
                    tocTooltip.style.top = (linkRect.bottom - sidebarRect.top + tocSidebar.scrollTop) + "px";
                    tocTooltip.classList.add("visible");
                });
                link.addEventListener("mouseleave", function() {
                    tocTooltip.classList.remove("visible");
                });
            })(tocLinks[i]);
        }

        if (tocLinks.length > 0) {
            // Build a map from heading id to TOC link
            var tocMap = {};
            var observedHeadings = [];
            for (var t = 0; t < tocLinks.length; t++) {
                var href = tocLinks[t].getAttribute("href");
                if (href && href.charAt(0) === "#") {
                    var targetId = href.substring(1);
                    var heading = document.getElementById(targetId);
                    if (heading) {
                        tocMap[targetId] = tocLinks[t];
                        observedHeadings.push(heading);
                    }
                }
            }

            if (observedHeadings.length > 0) {
                var currentActive = null;
                var suppressObserver = false;

                function activateTocLink(link) {
                    if (currentActive) currentActive.classList.remove("toc-active");
                    currentActive = link;
                    currentActive.classList.add("toc-active");
                    // Keep the active link visible in the sidebar (nearest, not centered)
                    var sidebarRect = tocSidebar.getBoundingClientRect();
                    var linkRect = link.getBoundingClientRect();
                    if (linkRect.top < sidebarRect.top) {
                        tocSidebar.scrollBy({ top: linkRect.top - sidebarRect.top - 10, behavior: "smooth" });
                    } else if (linkRect.bottom > sidebarRect.bottom) {
                        tocSidebar.scrollBy({ top: linkRect.bottom - sidebarRect.bottom + 10, behavior: "smooth" });
                    }
                }

                function centerTocLink(link) {
                    // Centre the link vertically in the sidebar
                    // (don't use scrollIntoView — it scrolls the whole page)
                    var sidebarRect = tocSidebar.getBoundingClientRect();
                    var linkRect = link.getBoundingClientRect();
                    var offset = linkRect.top - sidebarRect.top - (sidebarRect.height / 2) + (linkRect.height / 2);
                    tocSidebar.scrollBy({ top: offset, behavior: "smooth" });
                }

                // Suppress observer during smooth scroll (click or hash navigation)
                function suppressAndActivate(link) {
                    suppressObserver = true;
                    activateTocLink(link);
                    // Re-enable after scroll completes
                    if ("onscrollend" in window) {
                        document.addEventListener("scrollend", function() {
                            suppressObserver = false;
                        }, { once: true });
                    } else {
                        setTimeout(function() { suppressObserver = false; }, 1500);
                    }
                }

                var observer = new IntersectionObserver(function(entries) {
                    if (suppressObserver) return;
                    // Find the topmost visible heading
                    var topEntry = null;
                    for (var i = 0; i < entries.length; i++) {
                        if (entries[i].isIntersecting) {
                            if (!topEntry || entries[i].boundingClientRect.top < topEntry.boundingClientRect.top) {
                                topEntry = entries[i];
                            }
                        }
                    }
                    if (topEntry) {
                        var id = topEntry.target.getAttribute("id");
                        if (id && tocMap[id]) {
                            activateTocLink(tocMap[id]);
                        }
                    }
                }, { rootMargin: "0px 0px -80% 0px", threshold: 0 });

                // Clicking a TOC link: suppress observer and highlight immediately
                tocSidebar.addEventListener("click", function(e) {
                    var link = e.target.closest("a");
                    if (!link) return;
                    var href = link.getAttribute("href");
                    if (href && href.charAt(0) === "#") {
                        var id = decodeURIComponent(href.substring(1));
                        if (tocMap[id]) {
                            suppressAndActivate(tocMap[id]);
                        }
                    }
                });

                // On page load with a #hash, activate and centre immediately
                if (window.location.hash) {
                    var hashId = decodeURIComponent(window.location.hash.substring(1));
                    if (tocMap[hashId]) {
                        suppressAndActivate(tocMap[hashId]);
                        centerTocLink(tocMap[hashId]);
                    }
                }

                // Start observer
                for (var h = 0; h < observedHeadings.length; h++) {
                    observer.observe(observedHeadings[h]);
                }
            }
        }
    }

    // ===== Scrollable tables with sticky header + top scrollbar =====
    var tables = document.querySelectorAll(".col-content table");
    for (var ti = 0; ti < tables.length; ti++) {
        (function(table) {
            var parent = table.parentNode;
            // Only wrap tables that overflow their container
            if (table.scrollWidth <= parent.clientWidth) return;

            // Must have a thead to split
            var thead = table.querySelector("thead");
            if (!thead) return;

            // Measure column widths from the rendered table
            var firstRow = thead.querySelector("tr");
            if (!firstRow) return;
            var cells = firstRow.children;
            var widths = [];
            for (var c = 0; c < cells.length; c++) {
                widths.push(cells[c].getBoundingClientRect().width);
            }
            var tableWidth = table.scrollWidth;

            // Helper: create a colgroup with explicit widths
            function makeColgroup() {
                var cg = document.createElement("colgroup");
                for (var i = 0; i < widths.length; i++) {
                    var col = document.createElement("col");
                    col.style.width = widths[i] + "px";
                    cg.appendChild(col);
                }
                return cg;
            }

            // Clone table for the sticky header (thead only)
            var headerTable = table.cloneNode(true);
            var cloneBody = headerTable.querySelector("tbody");
            if (cloneBody) cloneBody.remove();
            headerTable.insertBefore(makeColgroup(), headerTable.firstChild);
            headerTable.style.width = tableWidth + "px";
            headerTable.style.minWidth = tableWidth + "px";

            // Lock column widths on the original table
            table.insertBefore(makeColgroup(), table.firstChild);
            table.style.width = tableWidth + "px";
            table.style.minWidth = tableWidth + "px";

            // Build: .table-scroll-wrap > (.table-scroll-header > inner > headerTable)
            //                            + (.table-scroll-body > table)
            var wrap = document.createElement("div");
            wrap.className = "table-scroll-wrap";

            var headerWrap = document.createElement("div");
            headerWrap.className = "table-scroll-header";
            var headerInner = document.createElement("div");
            headerInner.className = "table-scroll-header-inner";
            headerInner.appendChild(headerTable);
            headerWrap.appendChild(headerInner);

            var bodyWrap = document.createElement("div");
            bodyWrap.className = "table-scroll-body";

            // Scroll arrow buttons
            var arrowsDiv = document.createElement("div");
            arrowsDiv.className = "table-scroll-arrows";
            var leftBtn = document.createElement("button");
            leftBtn.className = "table-scroll-arrow";
            leftBtn.setAttribute("aria-label", "Scroll table left");
            leftBtn.innerHTML = '<svg aria-hidden="true"><use href="#icon-chevron-left"/></svg>';
            var rightBtn = document.createElement("button");
            rightBtn.className = "table-scroll-arrow";
            rightBtn.setAttribute("aria-label", "Scroll table right");
            rightBtn.innerHTML = '<svg aria-hidden="true"><use href="#icon-chevron-right"/></svg>';
            arrowsDiv.appendChild(leftBtn);
            arrowsDiv.appendChild(rightBtn);

            parent.insertBefore(wrap, table);
            bodyWrap.appendChild(table);
            wrap.appendChild(arrowsDiv);
            wrap.appendChild(headerWrap);
            wrap.appendChild(bodyWrap);

            // Update arrow visibility based on scroll position
            function updateArrows() {
                var scrollLeft = bodyWrap.scrollLeft;
                var maxScroll = bodyWrap.scrollWidth - bodyWrap.clientWidth;
                if (scrollLeft > 0) {
                    leftBtn.classList.add("visible");
                } else {
                    leftBtn.classList.remove("visible");
                }
                if (scrollLeft < maxScroll - 1) {
                    rightBtn.classList.add("visible");
                } else {
                    rightBtn.classList.remove("visible");
                }
            }
            updateArrows();

            // Build column edge positions, skipping the sticky first column
            var colEdges = [];
            var edge = 0;
            for (var ci = 0; ci < widths.length; ci++) {
                edge += widths[ci];
                if (ci > 0) colEdges.push(edge);
            }
            function scrollTo(target) {
                bodyWrap.scrollLeft = target;
                headerInner.scrollLeft = target;
                updateArrows();
            }
            leftBtn.addEventListener("click", function() {
                var pos = bodyWrap.scrollLeft;
                for (var ci = colEdges.length - 1; ci >= 0; ci--) {
                    if (colEdges[ci] < pos - 1) {
                        scrollTo(colEdges[ci]);
                        return;
                    }
                }
                scrollTo(0);
            });
            rightBtn.addEventListener("click", function() {
                var pos = bodyWrap.scrollLeft;
                for (var ci = 0; ci < colEdges.length; ci++) {
                    if (colEdges[ci] > pos + 1) {
                        scrollTo(colEdges[ci]);
                        return;
                    }
                }
            });

            // Sync horizontal scroll between header and body, update arrows
            var syncing = false;
            headerInner.addEventListener("scroll", function() {
                if (syncing) return;
                syncing = true;
                bodyWrap.scrollLeft = headerInner.scrollLeft;
                updateArrows();
                syncing = false;
            });
            bodyWrap.addEventListener("scroll", function() {
                if (syncing) return;
                syncing = true;
                headerInner.scrollLeft = bodyWrap.scrollLeft;
                updateArrows();
                syncing = false;
            });
        })(tables[ti]);
    }

    // ===== Copy to clipboard =====
    var copyIcon = '<svg class="icon" aria-hidden="true"><use href="#icon-copy"/></svg>';

    // Inject copy button into every pre block (outside pre, in a wrapper)
    var pres = document.querySelectorAll("pre");
    for (var p = 0; p < pres.length; p++) {
        var wrap = document.createElement("div");
        wrap.className = "pre-wrap";
        pres[p].parentNode.insertBefore(wrap, pres[p]);
        wrap.appendChild(pres[p]);
        var btn = document.createElement("button");
        btn.className = "copy-btn";
        btn.innerHTML = copyIcon;
        btn.setAttribute("type", "button");
        wrap.appendChild(btn);
    }

    // Single delegated handler for all copy buttons (.copy-btn)
    document.addEventListener("click", function(e) {
        var btn = e.target.closest(".copy-btn");
        if (!btn) return;

        // Determine what to copy
        var text;
        var pre = btn.closest("pre");
        if (pre) {
            // Code block: copy the code element text (excludes button)
            var code = pre.querySelector("code");
            text = (code || pre).textContent;
        } else {
            // Input field: copy value of the preceding input
            var input = btn.previousElementSibling;
            if (input && input.value) text = input.value;
        }
        if (!text) return;

        navigator.clipboard.writeText(text).then(function() {
            btn.classList.add("copied");
            setTimeout(function() {
                btn.classList.remove("copied");
            }, 2000);
        });
    });
})();

// ===== Google site search =====
function on_search() {
    document.search_form.q.value = document.search_form.words.value + " -site:forum.rclone.org";
    return true;
}
