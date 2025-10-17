# Copilot Instructions

## Commit Messages

When suggesting commit messages, always follow the conventional commit format combined with gitmoji emojis from https://gitmoji.dev/.

### Format
```TXT
:emoji: type(scope): description

[optional body]

[optional footer]
```

### Types
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `build`: Changes that affect the build system or external dependencies
- `ci`: Changes to our CI configuration files and scripts
- `chore`: Other changes that don't modify src or test files
- `revert`: Reverts a previous commit

### Emojis
Use the corresponding gitmoji for the type:
- ✨ feat
- 🐛 fix
- 📚 docs
- 💎 style
- ♻️ refactor
- ⚡ perf
- 🧪 test
- 🏗️ build
- 👷 ci
- 🧹 chore
- ⏪ revert

### Examples
- ✨ feat: add user authentication
- 🐛 fix(api): resolve null pointer exception in user service
- 📚 docs: update README with installation instructions
- ♻️ refactor: simplify user validation logic

Keep commit messages concise but descriptive. Use the imperative mood ("add" not "added").