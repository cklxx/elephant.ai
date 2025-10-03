# Publishing Guide

This project uses an `esbuild`-style multi-package approach to distribute the `alex` binary via npm. This involves a main package (`alex-code`) and several platform-specific packages (`@alex-code/linux-amd64`, etc.).

Publishing requires updating and publishing all of these packages.

## Prerequisites

1.  **Build all binaries**: Ensure you have run `make build-all` or that the binaries have been built by the CI/CD pipeline. The binaries must be available in the `build/` directory.
2.  **npm login**: You must be logged into an npm account that has permission to publish the `alex-code` and `@alex-code` scoped packages.

## Publishing Steps

1.  **Update Versions**:
    -   Increment the version number in the main package: `npm/alex-code/package.json`.
    -   Increment the version number in **all** platform-specific packages: `npm/alex-linux-amd64/package.json`, `npm/alex-darwin-arm64/package.json`, etc.
    -   Ensure the `optionalDependencies` in the main package's `package.json` point to the new correct version.

2.  **Copy Binaries to Packages**:
    -   You need to copy the compiled Go binaries from the `build/` directory into the `bin/` directory of each corresponding platform-specific npm package.
    -   For example, copy `build/alex-linux-amd64` to `npm/alex-linux-amd64/bin/alex`.
    -   A script could be created to automate this.

3.  **Publish Platform-Specific Packages**:
    -   Navigate into each platform-specific directory and run `npm publish`.
    -   It is important to publish all the platform packages **before** publishing the main package.
    -   Example:
        ```bash
        cd npm/alex-linux-amd64
        npm publish
        cd ../alex-darwin-arm64
        npm publish
        # ... repeat for all platforms
        ```

4.  **Publish the Main Package**:
    -   After all the platform-specific packages have been successfully published, navigate to the main package directory.
    -   Run `npm publish`.
        ```bash
        cd npm/alex-code
        npm publish
        ```

## Creating a new platform-specific package

If you need to support a new platform (e.g. `linux-riscv64`):

1.  Add the new platform to the build matrix in `.github/workflows/release.yml`.
2.  Create a new directory `npm/alex-linux-riscv64`.
3.  Create a `package.json` inside it with the correct `os` (`linux`) and `cpu` (`riscv64`) fields.
4.  Add the new package `@alex-code/linux-riscv64` to the `optionalDependencies` of the main `alex-code` package.
5.  Update the `install.js` script to recognize the new platform.
6.  Update this publishing guide.
