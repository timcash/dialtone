# Plan: video-slides-scrolling

## Goal
Implement a full-screen vertical scrolling (snap-to-slide) effect on the main page of `dialtone-earth`, featuring a D3 globe, a lazy-loaded video, and a Three.js graph scene.

## Tests
- [x] test_github_issue_created: Verify a GitHub issue is created using `dialtone-dev issue add`
- [x] test_layout_snap_css: Verify `layout.tsx` or `globals.css` includes scroll snap styles
- [x] test_home_page_sections: Verify `page.tsx` has three `<article>` or `<section>` elements for slides
- [x] test_globe_slide: Verify the first slide contains the D3 Globe component
- [x] test_video_slide: Verify the second slide contains a lazy-loaded `<video>` with `video1.mp4`
- [x] test_threejs_slide: Verify the third slide contains a Three.js scene with moving lines
- [x] test_regular_scrolling: Verify `/about` and `/docs` pages still use regular scrolling
- [x] test_puppeteer_scrolling: Run a Puppeteer test to verify scroll snap and component visibility

## Notes
- Scroll snap styles: `scroll-snap-type: y mandatory` on parent, `scroll-snap-align: start` on children.
- Lazy loading for video: Use `IntersectionObserver` in a `useEffect`.
- Three.js slide: Need a new component for the line graph.
- Navigation should probably be sticky or integrated into the layout.

## Blocking Issues
- None

## Progress Log
- 2026-01-16: Created plan file and defined initial goals and tests.
