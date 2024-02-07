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

    const upload = async (order) => {
        const id = new URL(order.href).searchParams.get('orderID')
        const res = await fetch('http://localhost:8080/api/purchases/' + id, {
            method: 'PUT',
            mode: 'cors',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({id, ...order})
        })
        x = {
            200: '👷',
            201: '👶',
            500: '🧟',
            409: '🙅'
        }
        return x[res.status] || '🤷'
    }

    const orders = await Promise.all(getInvoiceLinks().map(openInvoice))
    const results = await Promise.all(orders.map(upload))
    alert(results.join(' '))
    nextPage()
})()