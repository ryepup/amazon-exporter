# amazon-exporter

`amazon-exporter` is a ball of javascript to make it easier for me to do data
entry into YNAB.

It opens the "view invoice" link for each item in the order history, scrapes some data from the HTML, and finally opens a new window with a table summarizing every order, sorted by amount.

It integrates with the [YNAB API](https://api.ynab.com/) to match up amazon
purchases with unapproved transactions.

## Usage

1. install `docker` and `make`
2. (optional) create a [YNAB Personal Access Token](https://api.ynab.com/#personal-access-tokens), and put in into a `.env` file as `YNAB_TOKEN=$YOUR_TOKEN`
3. run `make serve`
4. open <http://localhost:8080> and follow the instructions

## Project goals

1. make my personal budgeting chores faster
2. vanilla js, no frontend build step (using libs from CDNs is OK)
3. easy installation

## See also

- <https://bulma.io/documentation/>
- <https://api.ynab.com/>
