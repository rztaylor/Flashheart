import type { ReactNode } from "react";

const sizes = {
  md: { text: "h-7 min-w-8 px-1.5 text-sm", image: "size-8" },
  lg: { text: "h-10 min-w-10 px-2 text-lg", image: "size-10" },
};

// KeyBadge is a project's key (FH) as a badge filled in the accent: the page
// header's identity and a project section's heading in Workstreams. With an
// image (the project's own, FH-151) it shows that instead, clipped to the
// badge's shape. Decorative; the project's name is beside it.
export function KeyBadge({
  children,
  size = "md",
  image,
}: {
  children: ReactNode;
  size?: keyof typeof sizes;
  // image is the URL of the project's image; the key is then not drawn.
  image?: string;
}) {
  return (
    <span
      aria-hidden="true"
      className={`grid shrink-0 place-items-center overflow-hidden rounded-control ${
        image
          ? sizes[size].image
          : `${sizes[size].text} bg-rail-active-mark text-on-rail-active-mark`
      }`}
    >
      {image ? (
        <img src={image} alt="" className="size-full object-cover" />
      ) : (
        children
      )}
    </span>
  );
}
