# ⚙️ inGitDB Repository Configuration - Languages

```yaml
# ⚙️ languages` list supported languages should preferably use `ISO 639-1`
# 📘 or alternatively `IETF BCP 47` (`RFC 5646` / `RFC 4647`).
# 📘 
# 📘 Examples:
# 📘 required: en
# 📘 required: es-ES
# 📘 required: es-MX
# 📘 optional: ru
# 📘 
# ⚙️ The required languages should be before the optional ones.
# ⚙️ Language selector will show languages in order defined in this list.
languages:
  - required: en
  - required: fr
  - required: es
  - optional: ru
```