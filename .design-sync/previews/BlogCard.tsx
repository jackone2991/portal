import { BlogCard } from "portal-frontend";

// Olympus blog-grid card: gradient cover with an overlaid category chip, title +
// excerpt, an avatar/author/date byline, and a heart/comment reaction footer.
// Framed on the app body surface at grid-cell width.

export const Default = () => (
  <div style={{ background: "#f5f6fa", padding: 16, maxWidth: 360 }}>
    <BlogCard
      title="The Majestic Canyon"
      excerpt="We hiked the north rim at dawn and watched the light pour into a mile of red sandstone."
      author="Marina Valentine"
      date="March 4, 2024"
      category="Travel"
      likes={248}
      comments={31}
    />
  </div>
);
