# Microsoft & Google Group Gating

## The Problem

Google OIDC and Microsoft OIDC (Entra ID) do not include group claims in their ID tokens by default. This means LM Gate's "Required Group" feature cannot directly read group membership from these providers' tokens.

## The Solution

Use **Authentik** or **Keycloak** as an intermediary identity provider. These platforms can:

1. Authenticate users via Google Workspace or Microsoft Entra ID (as upstream social login providers)
2. Sync group memberships from those platforms
3. Include group claims in their own OIDC tokens

The flow looks like:

```
Google Workspace / Microsoft Entra ID
        |
        v
Authentik or Keycloak (syncs groups, issues tokens with group claims)
        |
        v
LM Gate (reads groups from token, enforces Required Group)
```

## Setup Paths

### Authentik + Google Workspace

1. In Authentik, create a **Google OAuth Source** under Directory > Federation & Social Login
2. Configure it with your Google OAuth credentials
3. Set up group sync from Google Workspace via Authentik's Google Workspace provider
4. Create an OIDC application in Authentik for LM Gate
5. Ensure the `groups` claim is included in the Authentik OIDC token scope

See: [Authentik Google OAuth Source docs](https://docs.goauthentik.io/integrations/sources/google/)

### Authentik + Microsoft Entra ID

1. In Authentik, create a **Microsoft Entra ID OAuth Source**
2. Configure it with your Azure AD app registration credentials
3. Set up group sync from Entra ID
4. Create an OIDC application in Authentik for LM Gate
5. Ensure the `groups` claim is included in the token

See: [Authentik Azure AD Source docs](https://docs.goauthentik.io/integrations/sources/azure-ad/)

### Keycloak + Google Workspace

1. In Keycloak, add Google as an **Identity Provider** under the realm settings
2. Configure the Google OAuth client credentials
3. Set up group mappers to sync Google groups
4. Create an OIDC client in Keycloak for LM Gate
5. Add a group membership mapper to include groups in the ID token

See: [Keycloak Google Identity Provider docs](https://www.keycloak.org/docs/latest/server_admin/#google)

### Keycloak + Microsoft Entra ID

1. In Keycloak, add Microsoft as an **Identity Provider**
2. Configure the Azure AD app registration credentials
3. Set up group mappers to sync Entra ID groups
4. Create an OIDC client in Keycloak for LM Gate
5. Add a group membership mapper to include groups in the ID token

See: [Keycloak Microsoft Identity Provider docs](https://www.keycloak.org/docs/latest/server_admin/#microsoft)

## LM Gate Configuration

Once your intermediary provider is set up:

1. In LM Gate Settings, add an OIDC provider pointing to your Authentik or Keycloak instance
2. Set the **Groups Claim** to match the claim name used by your provider (typically `groups`)
3. Set the **Required Group** to the group name users must belong to (e.g., "LM Gate")
4. Only users whose token contains that group will be allowed to log in
