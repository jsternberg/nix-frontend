# Why Nix?

## What is Nix?

Nix is a dynamically weakly typed functional language with lazy evaluation and a syntax focused on package management.

It is dynamically typed because the person writing the code does not have to write out the types. It is weakly typed because there aren't many restrictions on converting between different types in different contexts. And, it is a functional language with lazy evaluation.

### What is lazy evaluation?

Lazy evaluation means that a section of code won't run unless it is actually used. For this frontend, that means that targets that aren't referenced won't have definitions created for them.

In most languages, creating a dictionary with two values would require both values to be evaluated.

```javascript
{
  "a": functionA(),
  "b": functionB(),
}
```

Both of these functions would be invoked to create the object even if the attribute `b` was never referenced.

In lazy evaluation, it is as-if every attribute was surrounded by a promise and was only evaluated when it was used.

### Why is that important?

This being part of the language means that complexity isn't passed on to the person using the frontend. They can just act as if lazy evaluation didn't exist and their definition would only do the minimum work required to produce the definition with no accidental side effects.

If we have a language builder for Go that produces a `vendor` target, this code is never run if that target is never used.

### Why not use the more common (insert declarative language) or (insert imperative language)?

When it comes to configuration and programming, there are generally two domains. Declarative languages are ones where you declare what you want and the runner figures out how to make what you have according to your specification. JSON, YAML, HCL, and other configuration languages are pretty common for this domain.

Then there are full Turing-complete programming languages like Javascript, Go, Lua, or any number of different scripting or programming languages.

It's natural to gravitate towards these languages for everything. They are usually the languages most programmers are more familiar with. At the same time, they have a difficult time striking the correct balance of declarative or imperative. Often, the declarative languages end up needing to include some imperative logic and it's difficult to retrofit conditional logic into these languages. That's how you end up with incredibly difficult interfaces such as Cloud Formation or meta languages like Ansible that are built on top of a common data format, but are essentially their own independent languages.

Then there are the full Turing-complete programming languages. The problem is these languages were built for a different purpose which either adds a lot of boilerplate to get started, hides a lot of details, or results in leaky abstractions.

Nix as a language is designed for the purpose of being the configuration language of a package manager and it strikes a good balance between full Turing-completeness and configuration. In most cases, you can use Nix the same way as if you were using JSON or YAML. But, if you suddenly need to include some logic, the functionality is already there.

Another possible language that fit this balance was [jsonnet](https://jsonnet.org/). I chose not to use jsonnet mostly because the JSON was more difficult to read than Nix even though it checked the same boxes that Nix did as a frontend language.
