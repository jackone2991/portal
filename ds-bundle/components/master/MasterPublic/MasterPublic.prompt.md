MasterPublic from portal-frontend. Use via `window.PortalUI.MasterPublic` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Guest / marketing shell — port of `master/master-public.blade.php`.
Minimal chrome: just the page body + the shared svg sprite.

## Props

```ts
interface MasterPublicProps {
children: React.ReactNode;
}
```

## Examples

### AuthCard

```jsx
() => (
  <MasterPublic>
    <div
      style={{
        minHeight: 560,
        display: "grid",
        placeItems: "center",
        padding: 24,
        background: "var(--tpl-bg, #f5f6fa)",
      }}
    >
      <div
        style={{
          width: 360,
          background: "#fff",
          borderRadius: 16,
          border: "1px solid var(--tpl-border, #e6ecf5)",
          boxShadow: "0 12px 40px rgba(63,66,87,0.10)",
          padding: 32,
        }}
      >
        <div
          style={{
            width: 48,
            height: 48,
            borderRadius: 12,
            marginBottom: 20,
            background:
              "linear-gradient(135deg, var(--tpl-accent, #ff5e3a), var(--tpl-accent-2, #ff763a))",
          }}
        />
        <h1
          style={{
            margin: 0,
            fontSize: 22,
            fontWeight: 700,
            color: "var(--tpl-heading, #3f4257)",
          }}
        >
          Welcome back
        </h1>
        <p style={{ margin: "6px 0 24px", fontSize: 14, color: "var(--tpl-muted, #888da8)" }}>
          Sign in to your Portal account
        </p>

        {[
          { label: "Email", value: "ada@portal.dev" },
          { label: "Password", value: "••••••••••" },
        ].map((f) => (
          <div key={f.label} style={{ marginBottom: 16 }}>
            <label
              style={{
                display: "block",
                fontSize: 12,
                fontWeight: 600,
                marginBottom: 6,
                color: "var(--tpl-text, #515365)",
              }}
            >
              {f.label}
            </label>
            <div
              style={{
                height: 42,
                display: "flex",
                alignItems: "center",
                padding: "0 12px",
                fontSize: 14,
                borderRadius: 10,
                color: "var(--tpl-text, #515365)",
                border: "1px solid var(--tpl-border, #e6ecf5)",
                background: "var(--tpl-surface-2, #f6f7fb)",
              }}
            >
              {f.value}
            </div>
          </div>
        ))}

        <button
          type="button"
          style={{
            width: "100%",
            height: 44,
            marginTop: 8,
            border: 0,
            borderRadius: 10,
            fontSize: 14,
            fontWeight: 700,
            color: "#fff",
            cursor: "pointer",
            background:
              "linear-gradient(135deg, var(--tpl-accent, #ff5e3a), var(--tpl-accent-2, #ff763a))",
          }}
        >
          Sign in
        </button>
        <p
          style={{
            margin: "18px 0 0",
            textAlign: "center",
            fontSize: 13,
            color: "var(--tpl-muted, #888da8)",
          }}
        >
          New here?{" "}
          <span style={{ fontWeight: 600, color: "var(--tpl-accent, #ff5e3a)" }}>
            Create an account
          </span>
        </p>
      </div>
    </div>
  </MasterPublic>
)
```
