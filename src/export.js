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

    const openInvoice = async (link) => {
        const w = window.open(link.href, '_blank')
        await new Promise((resolve) => w.addEventListener('load', resolve, true))
        const headings = [...w.document.querySelectorAll('td b')]

        const items = [...headings.filter(x => x.innerHTML === 'Items Ordered')[0]
            .closest('tbody')
            .querySelectorAll('td i')].map(x => x.innerText)

        const price = headings.map(x => x.innerText)
            .filter(x => x.startsWith('Order Total: $'))
            .map(x => parseFloat(x.replace('Order Total: $', '')))[0]

        const cc = headings.filter(x => x.innerText.startsWith('Credit Card transactions'))
        const charge = getCharge(cc)
        
        w.close()
        return {items, price, charge}
    }

    const itemHtml = (item) => `<li>${item}</li>`

    const itemsHtml = (items) => {
        return `<ul class="is-size-7">${items.map(itemHtml).join('')}<ul>`
    }

    const priceHtml = (price) => `$${price.toFixed(2)}`

    const chargeHtml = (charge) => {
        return `${charge.date} ${priceHtml(charge.amount)}`
    }

    const orderHtml = (order) => {
        const hasCharge = order.charge !== undefined
        const bg = hasCharge ? '' : 'has-background-danger-light'
        return `<tr class="${bg}">
            <td>${itemsHtml(order.items)}</td>
            <td>${priceHtml(order.price)}</td>
            <td>${hasCharge ? chargeHtml(order.charge) : '-'}</td>
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
                    <table class="table">
                        <thead>
                            <th>Items</th>
                            <th>Price</th>
                            <th>Charge</th>
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
        nextPage()
    }


    const orders = await Promise.all(getInvoiceLinks().map(openInvoice))
    output(orders)
})()