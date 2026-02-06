# Custom DNS Suffix Example

This example demonstrates how to configure a project to use a custom DNS suffix instead of the default `.test`.

## Setup

1.  **Initialize the project with the custom suffix:**
    ```bash
    cilo setup --name custom-dns --dns-suffix .localhost
    ```

2.  **Configure your system to resolve the custom suffix:**
    (Requires sudo)
    ```bash
    sudo cilo dns setup --dns-suffix .localhost
    ```

3.  **Create and start the environment:**
    ```bash
    cilo create dev
    cilo up dev
    ```

## Accessing the Environment

Once started, you can access your services using the custom suffix:

- **Web Service:** [http://web.dev.localhost](http://web.dev.localhost)
- **Project Apex:** [http://custom-dns.dev.localhost](http://custom-dns.dev.localhost)

## Why use a custom suffix?

- **Localhost Integration:** Using `.localhost` allows you to keep all your local development under the same TLD.
- **Company Standards:** Your organization might have a preferred TLD for development environments (e.g., `.local`, `.internal`).

## Troubleshooting

### Browser behavior
Some browsers (like Chrome) hardcode `*.localhost` to `127.0.0.1`. If `ping web.dev.localhost` works in your terminal but fails in the browser:
1. Try a different browser (like Firefox).
2. Or use a different suffix like `.test` or `.cilo`.

### System resolver
Verify that your system is correctly forwarding the suffix:
```bash
cilo dns status
```
The status command will attempt to detect the configured suffix and test resolution.
