# Subscribers – webhooks, emails, etc.

Subscribers are built-in configurable event-handlers that send notifications about inGitDB changes.

- Webhook – issues HTTP requests when changes happen in an inGitDB.
- Email – notifies user by email
- CLI - runs a local app

Here is an example of a `.ingitdb/subscribers.yaml`:

```yaml
subscribers:

  - name: Example of an email Subscriber
    email:
      from: ingitdb@example.com
      to:
        - alice@example.com
        - bob@example.com
      SMTP: smpt.example.com

  - name: Logger
    shell:
      run: |
        echo $1 $2 > log.txt

  - name: Example of a Webhooks Subscriber
    webhook:
      url: https://example.com/webhook/
```