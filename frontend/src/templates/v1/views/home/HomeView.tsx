/**
 * Home / newsfeed — port of `views/home/home.blade.php`.
 * (The Blade original was a red placeholder: "day la trang commic".)
 */
export function HomeView() {
  return (
    <section>
      <h1 className="text-2xl font-semibold">Newsfeed</h1>
      <p className="mt-2" style={{ color: "var(--tpl-muted)" }}>
        {/* TODO: post composer + feed list */}
        Your feed will appear here.
      </p>
    </section>
  );
}
