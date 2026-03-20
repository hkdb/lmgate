# Privacy Policy

**Last updated:** March 20, 2026

## Self-Hosted Software

LM Gate is **self-hosted** software. The project does not operate a hosted service and does not collect, transmit, or store your data on any external server.

## Your Data Stays Local

All data — including authentication credentials, audit logs, and usage metrics — is stored locally on your own infrastructure. The LM Gate project has no access to it.

## One Time Install Count Telemetry

On first startup, LM Gate sends a **single anonymous ping** to [Scarf](https://about.scarf.sh/) to track aggregate install counts. This ping:

- Is sent **once** per installation and recorded locally so it is not repeated
- Contains only the **application version number**
- Contains no personally identifiable information or usage data

Because the ping is served by Scarf (a third-party service), standard HTTP metadata such as your IP address may be processed by Scarf in accordance with their [privacy policy](https://about.scarf.sh/privacy-policy). LM Gate itself does not collect or store this information.

You can disable this telemetry by setting `LMGATE_TELEMETRY_DISABLED=true` in your `.env` file or `telemetry.disabled: true` in `config.yaml`.

## No Other Telemetry

Beyond the optional anonymous install count described above, LM Gate does **not** send any data to external servers, the author, or any third party.

## User Responsibility

If you deploy LM Gate, you are the data controller for your instance. You are responsible for:

- Your own privacy practices and data handling
- Compliance with applicable privacy regulations (GDPR, CCPA, etc.)
- Informing your own users about how their data is processed

## Commercial Deployments

When LM Gate is deployed as part of a commercial or paid service, the service provider is the data controller for their end users. The service provider is responsible for maintaining their own privacy policy that covers how end-user data is collected, processed, and stored. The LM Gate project bears no responsibility for how service providers handle end-user data.

## Third-Party Services

If you configure external integrations — such as OIDC providers, LLM APIs, or other third-party services — your use of those services is governed by their respective privacy policies. LM Gate does not control or take responsibility for third-party data practices.

## Changes to This Policy

This privacy policy may be updated at any time. Check this document for the latest version.
