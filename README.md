# amazon-exporter

`amazon-exporter` is a ball of javascript to make it easier for me to do data
entry into a budgeting program (I use YNAB).

It opens the "view invoice" link for each item in the order history, scrapes some data from the HTML, and finally opens a new window with a table summarizing every order, sorted by amount.

## Usage

1. install `docker` and `make`
2. setup a bookmarklet with <a href="javascript:(function(){var jsCode = document.createElement('script');jsCode.setAttribute('src', 'http://localhost:8080/export.js');document.body.appendChild(jsCode);}());)">this link</a>
3. run `make serve`
4. open the <https://www.amazon.com/your-orders>
5. run the bookmarklet
