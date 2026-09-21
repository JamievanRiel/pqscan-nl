// Looks up a domain in domains.json, the index generated with the site.
(() => {
  const form = document.getElementById("search");
  const input = document.getElementById("q");
  const out = document.getElementById("result");
  const labels = {
    "pq-default": ["Post-quantum by default", "Visitors with a current browser get a hybrid post-quantum key exchange."],
    "pq-supported": ["Supported, not default", "The server can do post-quantum key exchange but picks a classic one when a browser offers both."],
    "classic": ["Classic only", "The server only negotiated classic key exchange."],
    "unreachable": ["Not reachable", "No TLS handshake completed on port 443."],
  };

  let loaded = null;
  function load() {
    loaded ??= fetch("domains.json")
      .then((res) => {
        if (!res.ok) throw new Error("HTTP " + res.status);
        return res.json();
      })
      .then((data) => ({ date: data.date, entries: new Map(data.domains.map((d) => [d.d, d])) }));
    return loaded;
  }

  function normalize(q) {
    let d = q.trim().toLowerCase().replace(/^[a-z]+:\/\//, "").split("/")[0].replace(/\.$/, "");
    return d.startsWith("www.") ? d.slice(4) : d;
  }

  function show(domain, entry, date) {
    const card = document.createElement("div");
    card.className = "card";
    const title = document.createElement("h2");
    title.textContent = domain;
    card.append(title);
    if (!entry) {
      const p = document.createElement("p");
      p.textContent = "This domain was not scanned. Only .nl domains in the Tranco top 1 million and the domains on the sector lists are included.";
      card.append(p);
    } else {
      const [label, text] = labels[entry.s] || [entry.s, ""];
      const badge = document.createElement("p");
      badge.className = "badge status-" + entry.s;
      badge.textContent = label;
      const p = document.createElement("p");
      p.textContent = text;
      const meta = document.createElement("p");
      meta.className = "note";
      meta.textContent = [entry.h && "Host " + entry.h, entry.p && "network " + entry.p, "scanned " + date]
        .filter(Boolean)
        .join(" · ");
      card.append(badge, p, meta);
    }
    out.replaceChildren(card);
  }

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const domain = normalize(input.value);
    if (!domain) return;
    try {
      const { date, entries } = await load();
      show(domain, entries.get(domain), date);
    } catch (err) {
      loaded = null;
      out.textContent = "Could not load the results: " + err.message;
    }
  });

  const q = new URLSearchParams(location.search).get("q");
  if (q) {
    input.value = q;
    form.requestSubmit();
  }
})();
