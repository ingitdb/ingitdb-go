# Materialized views

Materialized views are used to build generated files based on ingitdb data.

For example you can have `cities` collection and might want to have few materialized views
for a quick load from a web app. Such as:

- Top 100 cities by population in descending order including population field value for each city
- Top 100 cities by alphabet (_id and name only_)
- All city ids like `['London', 'Mancheser', ...]`
