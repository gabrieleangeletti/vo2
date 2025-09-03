# Instructions

- Do never go straight to implementation. Always start with a plan and ask for confirmation before moving on.
- Read the most recently modified files content immediately before every single write or replace operation.
- Do not touch any code that is not related to the change you're working on.
- Do not rename anything unless it's directly related to the task at hand.
- Do not add any comments unless explicitly instructed to do so.
- Do not readjust the style of the code. We already have formatters in place, such as prettier.
- Do not change existing URLs in the code, unless explicitly instructed to do so.

Below you can find language-specific instructions.

[TypeScript]

- We never use `any`.
- Do not explictly write a type when it can be automatically inferred.
- Do not use `<Foo {...props} />` for React components. Instead, list out all props: `<Foo bar={props.bar} >`.
- Do not rename component props, unless explicitly asked to.
