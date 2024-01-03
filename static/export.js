(async function(){

    const getInvoiceLinks = () => Array.from(document.querySelectorAll('a[href^="/gp/css/summary/print.html"]'))
    const nextPage = () => document.querySelectorAll('li.a-last a[href^="/your-orders"]').forEach(x => x.click())

    const getCharge = (cc) => {
        if (cc.length === 0) {
            return undefined
        }
        const charge = cc[0].closest('tr').querySelector('td:nth-child(2)').innerText
        const [card, date, amount] = charge.split(':').map(x => x.trim())
        return {card, date, amount: parseFloat(amount.replace('$', ''))}
    }

    const getItems = (itemHeader) => Array.from(itemHeader.closest('tbody').querySelectorAll('td i'))
            .map(x => x.innerText)
    

    const openInvoice = async (link) => {
        const w = window.open(link.href, '_blank')
        await new Promise((resolve) => w.addEventListener('load', resolve, true))
        const headings = [...w.document.querySelectorAll('td b')]

        const items = headings.filter(x => x.innerHTML === 'Items Ordered')
            .map(getItems)
            .flat()

        const price = headings.map(x => x.innerText)
            .filter(x => x.startsWith('Order Total: $'))
            .map(x => parseFloat(x.replace('Order Total: $', '')))[0]

        const cc = headings.filter(x => x.innerText.startsWith('Credit Card transactions'))
        const charge = getCharge(cc)
        
        w.close()
        return {items, price, charge, href: link.href}
    }

    const itemHtml = (item) => `<li>${item}</li>`

    const itemsHtml = (items) => {
        return `<ul>${items.map(itemHtml).join('')}<ul>`
    }

    const priceHtml = (price) => `$${price.toFixed(2)}`

    const chargeHtml = (charge) => {
        return `<span class="is-size-7">
            ${charge.date}<br>${charge.card}<br>${priceHtml(charge.amount)}
        </span>`
    }

    const hrefHtml = (href) => {
        const id = new URL(href).searchParams.get('orderID')
        return `<a href="${href}" target="_blank">${id}</a>`
    }

    const upload = async (order) => {
        const id = new URL(order.href).searchParams.get('orderID')
        const res = await fetch('http://localhost:8080/api/purchases', {
            method: 'POST',
            mode: 'cors',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({id, ...order})
        })
        x = {200: '✅',
        500: '❌',
        409: '⚠️'
    }
        return {
            uploaded: x[res.status] || '?',
            ...order
        }
    }

    const orderHtml = (order) => {
        const hasCharge = order.charge !== undefined
        const bg = hasCharge ? '' : 'has-background-danger-light'
        return `<tr class="${bg}">
            <td>${hrefHtml(order.href)} (${order.uploaded})</td>
            <td>${itemsHtml(order.items)}</td>
            <td>${hasCharge ? chargeHtml(order.charge) : '-'}</td>
            <td>${priceHtml(order.price)}</td>
        </tr>`
    }

    const ordersHtml = (orders) => {
        return `<!DOCTYPE html>
        <html>
            <head>
            <meta charset="utf-8">
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <title>Order Summary</title>
            <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
            </head>
            <body>
                <section class="section">
                    <h1 class="title">Order Summary</h1>
                    <table class="table is-striped is-fullwidth">
                        <thead>
                            <th>Order</th>
                            <th>Items</th>
                            <th>Charge</th>
                            <th>Price</th>
                        </thead>
                        <tbody>
                            ${orders.map(orderHtml).join('')}
                        </tbody>
                    </table>
                </section>
            </body>
        </html>
        `
    }
    
    const output = (orders) => {
        orders.sort((a, b) => a.price < b.price)
        const body = URL.createObjectURL(new Blob([ordersHtml(orders)], {type: 'text/html'}))
        const w = window.open(body)
        console.log({orders})
        nextPage()
    }


    const orders = await Promise.all(getInvoiceLinks().map(openInvoice))
    output(await Promise.all(orders.map(upload)))
})()