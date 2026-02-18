# Materialized views

Materialized views are used to build generated files based on ingitdb data.

**There are three types of views:**

- Collection level views – basically a filter + lookup + renderer (to JSON/CSV/Markdown/etc.)
- Record level view – always takes a single record as an input and produces a child view.
- Root level views - can join multiple collections.

## Collection level views

For example you can have `cities` collection and might want to have few materialized views
for a quick load from a web app. Such as:

- Top 100 cities by population in descending order including population field value for each city
- Top 100 cities by alphabet (_id and name only_)
- All city ids like `['London', 'Mancheser', ...]`

## Record level views

Always takes a single record as an input
and produces a child view stored under the record.

Example:

- For each country shows a list of other countries with the same official language.

## Root level views

- Top 10 most populas cities in the world and their official language
- Top 10 French-speaking cities
