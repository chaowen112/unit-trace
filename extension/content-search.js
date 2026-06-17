// Runs on PropertyGuru search result pages.
// Finds all listing cards, batch-queries UnitTrace, injects status badges.

(function () {
  "use strict";

  const BADGE_ATTR = "data-ut-badge";

  function extractListingIdFromHref(href) {
    if (!href) return null;
    // Old: /property-listing/12345678-slug
    let m = href.match(/\/property-listing\/(\d+)/);
    if (m) return m[1];
    // New: /listing/slug-12345678
    m = href.match(/\/listing\/.*?-(\d{6,})(?:[^0-9]|$)/);
    if (m) return m[1];
    return null;
  }

  // Find all listing card anchors that have a listing ID in the href
  function findListingCards() {
    const cards = new Map(); // listingId → anchor element
    document.querySelectorAll('a[href*="/listing/"], a[href*="/property-listing/"]').forEach((a) => {
      const id = extractListingIdFromHref(a.getAttribute("href"));
      if (id && !cards.has(id)) {
        cards.set(id, a);
      }
    });
    return cards;
  }

  function statusColor(status) {
    switch (status) {
      case "seen_before": return { bg: "#e8f4ed", text: "#2a7d4f", border: "#b6dbc7" };
      case "likely_relisted": return { bg: "#fff7e6", text: "#d97706", border: "#fcd989" };
      case "possible_duplicate": return { bg: "#f3f0ff", text: "#7c3aed", border: "#c9b8fc" };
      default: return null;
    }
  }

  function statusLabel(status, data) {
    switch (status) {
      case "seen_before": {
        const parts = ["Seen before"];
        if (data.price_change_pct && data.price_change_pct < -0.1) {
          parts.push(`↓${Math.abs(data.price_change_pct).toFixed(1)}%`);
        }
        return parts.join(" · ");
      }
      case "likely_relisted": return "Likely relisted";
      case "possible_duplicate": return "Possible duplicate";
      default: return "";
    }
  }

  function injectStyles() {
    if (document.getElementById("ut-search-styles")) return;
    const style = document.createElement("style");
    style.id = "ut-search-styles";
    style.textContent = `
.ut-badge {
  display: inline-block;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 7px;
  border-radius: 4px;
  border: 1px solid;
  margin: 4px 0 0;
  white-space: nowrap;
  line-height: 1.4;
  pointer-events: none;
  user-select: none;
}
.ut-badge-wrap {
  display: block;
  margin-top: 4px;
}
    `;
    document.head.appendChild(style);
  }

  function injectBadge(anchor, listingId, status, data) {
    if (anchor.getAttribute(BADGE_ATTR) === listingId) return;
    anchor.setAttribute(BADGE_ATTR, listingId);

    const colors = statusColor(status);
    if (!colors) return;

    const label = statusLabel(status, data);
    if (!label) return;

    const span = document.createElement("span");
    span.className = "ut-badge-wrap";

    const badge = document.createElement("span");
    badge.className = "ut-badge";
    badge.textContent = label;
    badge.style.backgroundColor = colors.bg;
    badge.style.color = colors.text;
    badge.style.borderColor = colors.border;

    span.appendChild(badge);

    // Inject into the card — try common container patterns
    const container = anchor.closest('[class*="listing-card"]') ||
      anchor.closest('[class*="property-card"]') ||
      anchor.closest('[class*="result-item"]') ||
      anchor.closest("li") ||
      anchor;

    // Try to find a price element to inject after
    const priceEl = container.querySelector('[class*="price"]') ||
      container.querySelector('[class*="listing-title"]');

    if (priceEl) {
      priceEl.parentNode.insertBefore(span, priceEl.nextSibling);
    } else {
      // Fallback: inject right before the anchor ends
      anchor.appendChild(span);
    }
  }

  async function processBatch(cards) {
    const ids = Array.from(cards.keys());
    if (ids.length === 0) return;

    chrome.runtime.sendMessage(
      { type: "BATCH_STATUS", listing_ids: ids },
      (res) => {
        if (chrome.runtime.lastError || !res || !res.ok) return;
        const results = res.data.results || {};
        injectStyles();
        for (const [listingId, anchor] of cards) {
          const item = results[listingId];
          if (item && item.status && item.status !== "not_tracked") {
            injectBadge(anchor, listingId, item.status, item);
          }
        }
      }
    );
  }

  // Run once on load
  function run() {
    const cards = findListingCards();
    if (cards.size > 0) processBatch(cards);
  }

  // Also observe DOM changes for infinite-scroll / SPA navigation
  let debounceTimer = null;
  const observer = new MutationObserver(() => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      const cards = findListingCards();
      // Only process cards that haven't been badged yet
      const fresh = new Map();
      for (const [id, anchor] of cards) {
        if (anchor.getAttribute(BADGE_ATTR) !== id) fresh.set(id, anchor);
      }
      if (fresh.size > 0) processBatch(fresh);
    }, 600);
  });

  observer.observe(document.body, { childList: true, subtree: true });

  // Initial run after page settles
  setTimeout(run, 1200);
})();
