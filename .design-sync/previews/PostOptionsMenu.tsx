import { PostOptionsMenu } from "portal-frontend";
import { useEffect, useRef } from "react";

// The "…" menu on a post. Its open state is internal — there is no `open` prop —
// so a static render shows only the trigger and the menu itself is never seen.
//
// The preview therefore clicks the trigger once on mount. That drives the real
// component through its own public surface (a DOM click), rather than reaching
// into its state, so what the card shows is genuinely what a user sees.

function Opened({ items }: { items: { label: string; onSelect: () => void; danger?: boolean }[] }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    host.current?.querySelector("button")?.click();
  }, []);
  return (
    <div ref={host} className="flex justify-end">
      <PostOptionsMenu items={items} />
    </div>
  );
}

const stage = (children: React.ReactNode) => (
  <div
    data-template="v1"
    className="rounded-xl p-4"
    style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)", width: 320, minHeight: 210 }}
  >
    {children}
  </div>
);

export const OwnPost = () =>
  stage(
    <Opened
      items={[
        { label: "Sửa bài", onSelect: () => {} },
        { label: "Ghim lên đầu", onSelect: () => {} },
        { label: "Sao chép liên kết", onSelect: () => {} },
        { label: "Xoá bài", onSelect: () => {}, danger: true },
      ]}
    />,
  );

// Someone else's post: no destructive item, so the danger colour is absent —
// the menu is built from what the caller is allowed to do, not a fixed list.
export const OthersPost = () =>
  stage(
    <Opened
      items={[
        { label: "Sao chép liên kết", onSelect: () => {} },
        { label: "Báo cáo bài viết", onSelect: () => {} },
      ]}
    />,
  );
