(async function () {
  const findDate = (row) => {
    let el = row.parentElement.previousElementSibling;
    while (el && !el.classList.contains("apx-transaction-date-container")) {
      el = el.previousElementSibling;
    }
    return el?.innerText.trim();
  };

  const getTransactions = () => {
    const rows = document.querySelectorAll(
      ".apx-transactions-line-item-component-container"
    );

    return Array.from(rows)
      .filter((row) => row.children.length > 0)
      .map((row) => {
        const link = row.querySelector("a");
        const id = link.innerText.replace("Order #", "").trim();
        const href = id.startsWith("D")
          ? link.href + "&print=1"
          : link.href.replace("edit.html", "print.html");

        const amount = parseFloat(
          row.querySelector(".a-span-last").innerText.replace("$", "")
        );
        return {
          id,
          href,
          charge: {
            amount,
            date: findDate(row),
            card: row.querySelector(".a-text-bold").innerText.trim(),
          },
        };
      });
  };

  const withNewWindow = async (url, fn) => {
    const w = window.open(url, "_blank");
    await new Promise((resolve) => w.addEventListener("load", resolve, true));
    const ret = fn(w.document);
    w.close();
    return ret;
  };

  const getItems = (itemHeader) =>
    Array.from(itemHeader.closest("tbody").querySelectorAll("td i")).map(
      (x) => x.innerText
    );

  const scrapeSubscribeAndSave = (doc) => {
    const headings = Array.from(doc.querySelectorAll("td b"));

    const price = headings
      .map((x) => x.innerText)
      .filter((x) => x.startsWith("Order Total: $"))
      .map((x) => parseFloat(x.replace("Order Total: $", "")))[0];

    const items = headings
      .filter((x) => x.innerHTML === "Items Ordered")
      .map(getItems)
      .flat();

    return { price, items };
  };

  const scrapeInvoice = (doc) => {
    const isSubscribeAndSave = Array.from(doc.querySelectorAll("b")).some(
      (el) => el.innerText.startsWith("Subscribe and Save")
    );

    if (isSubscribeAndSave) {
      return scrapeSubscribeAndSave(doc);
    }

    const price = parseFloat(
      doc
        .querySelector(".od-line-item-row:last-child .a-span-last")
        .innerText.replace("$", "")
    );

    const items = Array.from(
      doc.querySelectorAll('[data-component="itemTitle"]')
    ).map((el) => el.innerText.trim());

    return { items, price };
  };

  const scrapeDigitalInvoice = (doc) => {
    const price = parseFloat(
      doc.querySelector(".a-color-price").innerText.replace("$", "")
    );

    const items = [doc.querySelector('td[valign="top"]').innerText.trim()];

    return { items, price };
  };

  const openInvoice = async (order) => {
    const details = await withNewWindow(
      order.href,
      order.id.startsWith("D") ? scrapeDigitalInvoice : scrapeInvoice
    );
    return { ...order, ...details };
  };

  const upload = async (order) => {
    const res = await fetch("http://localhost:8080/api/purchases/" + order.id, {
      method: "PUT",
      mode: "cors",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(order),
    });
    x = {
      200: "👷",
      201: "👶",
      500: "🧟",
      409: "🙅",
    };
    return x[res.status] || "🤷";
  };

  const orders = await Promise.all(getTransactions().map(openInvoice));
  const results = await Promise.all(orders.map(upload));
  alert(results.join(" "));
  document.querySelector(".a-span-last .a-button-input").click();
})();
