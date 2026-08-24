Composer from portal-frontend. Use via `window.PortalUI.Composer` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Empty

```jsx
() =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd=""
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
    />,
  )
```

### Drafting

```jsx
() =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd="Vừa đọc xong chương mới của Vạn Cổ Thần Đế — nhịp truyện nhanh hơn hẳn arc trước."
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
    />,
  )
```

### Submitting

```jsx
() =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd="Đang đăng ghi chú này…"
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
      submitting
    />,
  )
```

### WithError

```jsx
() =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd="Ghi chú không gửi được."
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
      error="Không đăng được — thử lại sau."
    />,
  )
```
