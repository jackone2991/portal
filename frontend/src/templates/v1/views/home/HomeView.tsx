"use client";

import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Avatar } from "../../components/ui/Avatar";
import { Icon } from "../../components/ui/Icon";

/**
 * Home / newsfeed — React + Tailwind port of the Olympus "Newsfeed"
 * (template-main/social/social/Newsfeed.html), light theme, authentic sprite icons.
 *
 * Left rail: weather · calendar · pages. Centre: composer + post feed.
 * Right rail: story card · friend suggestions · activity feed.
 *
 * Feed content mirrors the Olympus sample; there is no social/posts backend in
 * v1, so posting from the composer optimistically prepends to the local list.
 */

const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "https://api.portal.localhost";

type Media = { title: string; desc: string; source: string };

type Post = {
  id: string;
  author: string;
  action: ReactNode;
  time: string;
  text: ReactNode;
  media?: Media;
  likes: number;
  likedByLabel: ReactNode;
  comments: number;
  shares: number;
  liked?: boolean;
};

const INITIAL_FEED: Post[] = [
  {
    id: "p1",
    author: "Marina Valentine",
    action: (
      <>
        shared a <A>link</A>
      </>
    ),
    time: "March 4 at 2:05pm",
    text: (
      <>
        Hey <A>Cindi</A>, you should really check out this new song by Iron Maid. The next time they
        come to the city we should totally go!
      </>
    ),
    media: {
      title: "Iron Maid - ChillGroves",
      desc: "Lorem ipsum dolor sit amet, consectetur ipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua...",
      source: "YOUTUBE.COM",
    },
    likes: 18,
    likedByLabel: (
      <>
        <b>Jenny</b>, <b>Robert</b> and <b>18 more</b> liked this
      </>
    ),
    comments: 0,
    shares: 16,
  },
  {
    id: "p2",
    author: "Elaine Dreyfuss",
    action: "",
    time: "9 hours ago",
    text: "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempo incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris consequat.",
    likes: 24,
    likedByLabel: (
      <>
        <b>You</b>, <b>Elaine</b> and <b>22 more</b> liked this
      </>
    ),
    comments: 17,
    shares: 24,
  },
  {
    id: "p3",
    author: "James Spiegel",
    action: "",
    time: "38 mins ago",
    text: "Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium der doloremque laudantium, totam rem aperiam.",
    likes: 12,
    likedByLabel: (
      <>
        <b>Nicholas</b>, <b>Diana</b> and <b>10 more</b> liked this
      </>
    ),
    comments: 3,
    shares: 5,
  },
];

export function HomeView() {
  const [displayName, setDisplayName] = useState("You");
  const [posts, setPosts] = useState<Post[]>(INITIAL_FEED);

  useEffect(() => {
    let alive = true;
    fetch(`${API_BASE}/api/v1/auth/me`, { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((me: { display_name?: string } | null) => {
        if (alive && me?.display_name) setDisplayName(me.display_name);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  function handlePost(text: string) {
    setPosts((prev) => [
      {
        id: `me-${prev.length}-${text.length}`,
        author: displayName,
        action: "",
        time: "Just now",
        text,
        likes: 0,
        likedByLabel: <span style={{ color: "var(--tpl-muted)" }}>Be the first to like this</span>,
        comments: 0,
        shares: 0,
      },
      ...prev,
    ]);
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[260px_minmax(0,1fr)] 2xl:grid-cols-[260px_minmax(0,1fr)_300px]">
      <div className="hidden space-y-5 lg:block">
        <WeatherWidget />
        <CalendarWidget />
        <PagesWidget />
      </div>

      <div className="min-w-0 space-y-5">
        <Composer displayName={displayName} onPost={handlePost} />
        {posts.map((p) => (
          <PostCard
            key={p.id}
            post={p}
            onToggleLike={() =>
              setPosts((prev) =>
                prev.map((x) =>
                  x.id === p.id
                    ? { ...x, liked: !x.liked, likes: x.likes + (x.liked ? -1 : 1) }
                    : x,
                ),
              )
            }
          />
        ))}
      </div>

      <aside className="hidden space-y-5 2xl:block">
        <BirthdayCard />
        <FriendSuggestions />
        <ActivityFeed />
      </aside>
    </div>
  );
}

/* ── Left rail widgets ────────────────────────────────────────────── */

function WeatherWidget() {
  const week = [
    ["SUN", "weather-sunny-icon", 60],
    ["MON", "weather-sunny-icon", 58],
    ["TUE", "weather-cloudy-icon", 67],
    ["WED", "weather-rain-icon", 70],
    ["THU", "weather-rain-icon", 58],
    ["FRI", "weather-rain-icon", 68],
    ["SAT", "weather-partly-sunny-icon", 65],
  ] as const;
  return (
    <div
      className="overflow-hidden rounded-xl text-white shadow-sm"
      style={{ background: "linear-gradient(160deg, var(--tpl-weather-1), var(--tpl-weather-2))" }}
    >
      <div className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-1">
            <span className="text-5xl font-light leading-none">64°</span>
            <span className="mt-1 text-xs leading-tight text-white/80">
              58°
              <br />
              76°
            </span>
          </div>
          <Icon name="weather-partly-sunny-icon" size={40} />
        </div>
        <p className="mt-3 text-lg font-semibold">Partly Sunny</p>
        <p className="text-xs text-white/80">Real Feel: 67°&nbsp;&nbsp;&nbsp;Chance of Rain: 49%</p>
      </div>

      <div className="grid grid-cols-7 gap-1 px-3 py-4" style={{ background: "rgba(255,255,255,0.08)" }}>
        {week.map(([d, ic, t]) => (
          <div key={d} className="flex flex-col items-center gap-1.5 text-[10px] text-white/85">
            <span className="font-semibold">{d}</span>
            <Icon name={ic} size={18} />
            <span>{t}°</span>
          </div>
        ))}
      </div>

      <div className="px-5 py-3 text-center text-xs">
        <p className="font-semibold">Saturday, March 26th</p>
        <p className="text-white/75">San Francisco, CA</p>
      </div>
    </div>
  );
}

function CalendarWidget() {
  const days = Array.from({ length: 31 }, (_, i) => i + 1);
  return (
    <WidgetCard>
      <div className="mb-3 flex items-center justify-between px-1" style={{ color: "var(--tpl-muted)" }}>
        <button type="button" aria-label="Previous month">
          <Icon name="popup-left-arrow" size={12} />
        </button>
        <span className="font-semibold" style={{ color: "var(--tpl-heading)" }}>
          May
        </span>
        <button type="button" aria-label="Next month">
          <Icon name="popup-right-arrow" size={12} />
        </button>
      </div>
      <div className="grid grid-cols-7 gap-y-2 text-center text-[11px]">
        {["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"].map((d) => (
          <span key={d} className="font-semibold" style={{ color: "var(--tpl-muted)" }}>
            {d}
          </span>
        ))}
        {days.map((n) => (
          <span
            key={n}
            className="grid h-7 place-items-center rounded-full text-xs"
            style={
              n === 12
                ? { background: "var(--tpl-accent)", color: "#fff", margin: "0 auto", width: 28 }
                : { color: "var(--tpl-text)" }
            }
          >
            {n}
          </span>
        ))}
      </div>
    </WidgetCard>
  );
}

function PagesWidget() {
  const pages = [
    { name: "The Marina Bar", cat: "Restaurant · Bar" },
    { name: "Tapronus Rock", cat: "Rock Band" },
    { name: "Pixel Digital Design", cat: "Company" },
    { name: "Thompson's Custom Clothing Boutique", cat: "Clothing Store" },
  ];
  return (
    <WidgetCard title="Pages You May Like" more>
      <ul className="space-y-3">
        {pages.map((p) => (
          <li key={p.name} className="flex items-center gap-3">
            <Avatar name={p.name} size={38} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {p.name}
              </p>
              <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                {p.cat}
              </p>
            </div>
            <button
              type="button"
              className="ml-auto text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
              aria-label="Like page"
            >
              <Icon name="star-icon" size={18} />
            </button>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

/* ── Composer ─────────────────────────────────────────────────────── */

const TABS = [
  { key: "status", label: "Status", icon: "status-icon" },
  { key: "media", label: "Multimedia", icon: "multimedia-icon" },
  { key: "blog", label: "Blog Post", icon: "blog-icon" },
];

function Composer({
  displayName,
  onPost,
}: {
  displayName: string;
  onPost: (text: string) => void;
}) {
  const [tab, setTab] = useState("status");
  const [text, setText] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    const t = text.trim();
    if (!t) return;
    onPost(t);
    setText("");
  }

  return (
    <Card className="overflow-hidden">
      <div className="flex border-b" style={{ borderColor: "var(--tpl-border)" }}>
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            className="flex items-center gap-2 px-5 py-3.5 text-sm font-semibold transition"
            style={
              tab === t.key
                ? { color: "var(--tpl-accent)", boxShadow: "inset 0 -2px 0 var(--tpl-accent)" }
                : { color: "var(--tpl-muted)" }
            }
          >
            <Icon name={t.icon} size={16} />
            <span className="hidden sm:inline">{t.label}</span>
          </button>
        ))}
      </div>

      <form onSubmit={submit} className="p-4">
        <div className="flex gap-3">
          <Avatar name={displayName} size={40} />
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={2}
            placeholder="Share what you are thinking here..."
            className="min-h-[3rem] w-full resize-none border-0 bg-transparent pt-2 text-sm outline-none placeholder:text-[var(--tpl-muted)]"
            style={{ color: "var(--tpl-text)" }}
          />
        </div>

        <div className="mt-2 flex items-center gap-1 border-t pt-3" style={{ borderColor: "var(--tpl-border)" }}>
          <IconBtn label="Add photos" icon="camera-icon" />
          <IconBtn label="Tag friends" icon="computer-icon" />
          <IconBtn label="Add location" icon="small-pin-icon" />

          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              Preview
            </button>
            <button
              type="submit"
              disabled={!text.trim()}
              className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            >
              Post Status
            </button>
          </div>
        </div>
      </form>
    </Card>
  );
}

/* ── Post card ────────────────────────────────────────────────────── */

function PostCard({ post, onToggleLike }: { post: Post; onToggleLike: () => void }) {
  return (
    <Card as="article" className="relative p-5">
      <div className="absolute right-0 top-6 hidden translate-x-1/2 flex-col gap-2 sm:flex">
        <QuickFab active={post.liked} onClick={onToggleLike} label="Like" icon="like-post-icon" />
        <QuickFab label="Comment" icon="comments-post-icon" />
        <QuickFab label="Share" icon="share-post-icon" />
      </div>

      <header className="flex items-center gap-3">
        <Avatar name={post.author} size={40} />
        <div className="min-w-0">
          <p className="truncate text-sm">
            <span className="font-semibold" style={{ color: "var(--tpl-heading)" }}>
              {post.author}
            </span>{" "}
            {post.action && <span style={{ color: "var(--tpl-muted)" }}>{post.action}</span>}
          </p>
          <time className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            {post.time}
          </time>
        </div>
        <button type="button" className="ml-auto pr-8 text-[var(--tpl-muted)]" aria-label="Post options">
          <Icon name="three-dots-icon" size={18} />
        </button>
      </header>

      <p className="mt-3 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
        {post.text}
      </p>

      {post.media && <VideoCard media={post.media} />}

      <footer className="mt-4 flex items-center gap-3 text-sm">
        <span
          className="inline-flex items-center gap-2"
          style={{ color: post.liked ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
        >
          <Icon name="heart-icon" size={16} />
          <span>{post.likes}</span>
        </span>
        <StackedAvatars names={["Jenny R", "Robert K", "Amy L", "Leo M"]} />
        <span className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
          {post.likedByLabel}
        </span>
        <span className="ml-auto inline-flex items-center gap-4" style={{ color: "var(--tpl-muted)" }}>
          <span className="inline-flex items-center gap-1.5">
            <Icon name="speech-balloon-icon" size={16} />
            <span className="text-xs">{post.comments}</span>
          </span>
          <span className="inline-flex items-center gap-1.5">
            <Icon name="share-icon" size={16} />
            <span className="text-xs">{post.shares}</span>
          </span>
        </span>
      </footer>
    </Card>
  );
}

function VideoCard({ media }: { media: Media }) {
  return (
    <div className="mt-3 flex gap-4 overflow-hidden rounded-lg border" style={{ borderColor: "var(--tpl-border)" }}>
      <div
        className="relative grid h-40 w-44 shrink-0 place-items-center"
        style={{ background: "linear-gradient(135deg,#c78a5b,#7c5240)" }}
      >
        <span
          className="grid h-14 w-14 place-items-center rounded-full text-white shadow"
          style={{ background: "var(--tpl-accent)" }}
        >
          <Icon name="play-icon" size={20} />
        </span>
      </div>
      <div className="min-w-0 py-4 pr-4">
        <p className="text-base font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {media.title}
        </p>
        <p className="mt-1 text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
          {media.desc}
        </p>
        <p className="mt-3 text-[11px] font-semibold tracking-wide" style={{ color: "var(--tpl-muted)" }}>
          {media.source}
        </p>
      </div>
    </div>
  );
}

/* ── Right rail widgets ───────────────────────────────────────────── */

function BirthdayCard() {
  return (
    <div
      className="relative overflow-hidden rounded-xl p-5 text-white shadow-sm"
      style={{ background: "linear-gradient(150deg, #8a63d2, #6d4bb8)" }}
    >
      {/* decorative blobs (≈ Olympus birthday widget) */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-20"
        style={{
          background:
            "radial-gradient(60% 60% at 85% 15%, rgba(255,255,255,0.5), transparent 60%)," +
            "radial-gradient(50% 50% at 20% 90%, rgba(255,255,255,0.25), transparent 60%)",
        }}
      />
      <div className="relative">
        <div className="flex items-start justify-between">
          <Icon name="cupcake-icon" size={22} />
          <button type="button" aria-label="Options">
            <Icon name="three-dots-icon" size={18} />
          </button>
        </div>

        <div className="mt-4">
          <Avatar name="Marina Valentine" size={44} className="ring-2 ring-white/40" />
        </div>

        <p className="mt-3 text-sm text-white/80">Today is</p>
        <h3 className="text-2xl font-bold leading-tight">Marina Valentine&apos;s Birthday!</h3>
        <p className="mt-3 text-sm leading-relaxed text-white/80">
          Leave her a message with your best wishes on her profile page!
        </p>
      </div>
    </div>
  );
}

function FriendSuggestions() {
  const people = [
    { name: "Francine Smith", meta: "8 Friends in Common" },
    { name: "Hugh Wilson", meta: "6 Friends in Common" },
    { name: "Karen Masters", meta: "6 Friends in Common" },
  ];
  return (
    <WidgetCard title="Friend Suggestions" more>
      <ul className="space-y-3">
        {people.map((p) => (
          <li key={p.name} className="flex items-center gap-3">
            <Avatar name={p.name} size={40} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {p.name}
              </p>
              <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                {p.meta}
              </p>
            </div>
            <button
              type="button"
              className="ml-auto grid h-8 w-8 place-items-center rounded-md text-white transition hover:opacity-90"
              style={{ background: "var(--tpl-blue)" }}
              aria-label={`Add ${p.name}`}
            >
              <Icon name="happy-face-icon" size={16} />
            </button>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

function ActivityFeed() {
  const items = [
    { who: "Marina Polson", what: <>commented on Jason Mark&apos;s <A>photo</A>.</>, time: "2 mins ago", icon: "comments-post-icon" },
    { who: "Jake Parker", what: <>liked Nicholas Grissom&apos;s <A>status update</A>.</>, time: "5 mins ago", icon: "like-post-icon" },
    { who: "Mary Jane Stark", what: <>added 20 new photos to her <A>gallery album</A>.</>, time: "12 mins ago", icon: "photos-icon" },
    { who: "Nicholas Grissom", what: <>updated his profile <A>photo</A>.</>, time: "1 hour ago", icon: "happy-face-icon" },
  ];
  return (
    <WidgetCard title="Activity Feed">
      <ul className="space-y-4">
        {items.map((it) => (
          <li key={it.who} className="flex items-start gap-3">
            <Avatar name={it.who} size={36} />
            <div className="min-w-0 flex-1">
              <p className="text-sm leading-snug" style={{ color: "var(--tpl-text)" }}>
                <b style={{ color: "var(--tpl-heading)" }}>{it.who}</b> {it.what}
              </p>
              <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                {it.time}
              </p>
            </div>
            <span className="mt-1 shrink-0" style={{ color: "var(--tpl-muted)" }}>
              <Icon name={it.icon} size={16} />
            </span>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

/* ── Shared pieces ────────────────────────────────────────────────── */

function Card({
  children,
  className = "",
  as: Tag = "div",
}: {
  children: ReactNode;
  className?: string;
  as?: "div" | "article";
}) {
  return (
    <Tag
      className={`rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {children}
    </Tag>
  );
}

function WidgetCard({
  title,
  more,
  children,
}: {
  title?: string;
  more?: boolean;
  children: ReactNode;
}) {
  return (
    <Card className="p-4">
      {title && (
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>
            {title}
          </h3>
          {more && (
            <span style={{ color: "var(--tpl-muted)" }}>
              <Icon name="three-dots-icon" size={18} />
            </span>
          )}
        </div>
      )}
      {children}
    </Card>
  );
}

function A({ children }: { children: ReactNode }) {
  return (
    <a href="#" className="font-medium hover:underline" style={{ color: "var(--tpl-accent)" }}>
      {children}
    </a>
  );
}

function StackedAvatars({ names }: { names: string[] }) {
  return (
    <span className="flex">
      {names.map((n, i) => (
        <span key={n} className="rounded-full ring-2 ring-white" style={{ marginLeft: i ? -8 : 0 }}>
          <Avatar name={n} size={22} />
        </span>
      ))}
    </span>
  );
}

function IconBtn({ label, icon }: { label: string; icon: string }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className="grid h-9 w-9 place-items-center rounded-lg transition hover:bg-[var(--tpl-surface-2)]"
      style={{ color: "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={18} />
    </button>
  );
}

function QuickFab({
  onClick,
  active,
  label,
  icon,
}: {
  onClick?: () => void;
  active?: boolean;
  label: string;
  icon: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="grid h-8 w-8 place-items-center rounded-full border bg-white shadow-sm transition hover:text-[var(--tpl-accent)]"
      style={{ borderColor: "var(--tpl-border)", color: active ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={15} />
    </button>
  );
}
