# dotenv files

The dotenv files (.env, .env.development, etc) give binaries a consistent configuration story instead of letting each command surface invent its own rules for:

- default values
- dotenv loading
- environment-variable overrides
- command-line flag overrides

By centralizing configuration here, the project keeps configuration behavior:

- explicit
- reusable
- testable
- consistent across entrypoints

## Configuration precedence

The intended precedence is:

1. hard-coded defaults
2. shared dotenv files
3. local dotenv override files
4. real environment variables
5. command-line flags

## Dotenv Policy

The project distinguishes between shared dotenv files and local override files.

Shared dotenv files:

- `.env`
- `.env.development`
- `.env.production`
- `.env.test`

Local override files:

- `.env.local`
- `.env.development.local`
- `.env.production.local`
- `.env.test.local`

Shared files describe project defaults for a given environment. Local override files describe machine-specific or secret-bearing overrides.

For the design reasoning behind this policy, see [../../docs/explanations/dotenv-files-and-configuration-precedence.md](/Users/wraith/Software/playbymail/drynn/docs/explanations/dotenv-files-and-configuration-precedence.md).

## dotenv Precedence

The dotenv package loads environment files in a specific order.

### Development Environment

| 	Priority______________ |.gitignore| Secrets?| Notes________________________________|
|------------------------------|----------|---------|--------------------------------------|
| 	.env.development.local |Always    | Yes     | Environment-specific local overrides|
| 	.env.local             |Always    | Yes     | Local overrides|
| 	.env.development       |No        | Never   | Shared environment-specific variables|
| 	.env                   |No        | Never   | Shared for all environments|

### Test Environment

| 	Priority______________ |.gitignore| Secrets?| Notes________________________________|
|------------------------------|----------|---------|--------------------------------------|
| 	.env.test.local        |Always    | Yes     | Environment-specific local overrides|
| 	.env.test              |No        | Never   | Shared environment-specific variables|
| 	.env                   |No        | Never   | Shared for all environments|

### Production Environment

| 	Priority______________ |.gitignore| Secrets?| Notes________________________________|
|------------------------------|----------|---------|--------------------------------------|
| 	.env.production.local  |Always    | Yes     | Environment-specific local overrides|
| 	.env.local             |Always    | Yes     | Local overrides|
| 	.env.production        |No        | Never   | Shared environment-specific variables|
| 	.env                   |No        | Never   | Shared for all environments|


## Maintenance Notes

When adding new configuration:

- decide explicitly whether the setting belongs in code defaults, shared env files, local overrides, environment variables, or flags
- preserve a single precedence model across all binaries

If configuration behavior becomes more complex later, this project should grow more structure. For now, the priority is one clear place for configuration rules.
