# VAR Method

Naming-suffix convention. The most specific layer of the naming rules in
`code_conventions.md`, and the lowest in its precedence order. This is a guide,
not a binding rule: apply a suffix only when it describes the real responsibility,
and prefer a name that reveals responsibility on its own (already required by
`code_conventions.md` Naming) over any suffix. When the specific name is clear, the
suffix is redundant; drop it.

## Benefits

- Self-explanatory, direct code.
- Clear responsibility per variable or class.
- Standardized structure for easier maintenance and reading.

## Suffixes

- Data: raw data, payloads, simple object attributes (userData, paymentData).
- Info: processed data, descriptive summaries, configuration (systemInfo, accountInfo).
- Manager: complex classes or objects orchestrating processes, states, connections (SessionManager). Use sparingly. A specific name almost always beats this suffix; it is easily overused for classes that do too much, which is itself a design smell to fix rather than name.
- Handler: functions reacting to specific events such as user actions or errors (onClickHandler). Same overuse caveat as Manager.

The boundary between Data and Info is a gradient, not a hard line; do not agonize
over it. If a suffix does not make the responsibility clearer, omit it.

## Examples

Data
```javascript
const userData = {
  id: 1,
  name: "Lucas",
  email: "lucas@email.com"
};
```

Info
```javascript
const systemInfo = {
  os: "Linux",
  version: "1.0.4",
  environment: "production"
};
```

Manager
```javascript
class SessionManager {
  login(credentials) { /* logic */ }
  logout() { /* logic */ }
  validateToken() { /* logic */ }
}
```

Handler
```javascript
function submitFormHandler(event) {
  event.preventDefault();
  /* submission logic */
}
```
