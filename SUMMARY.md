# login-goose-logo

## What was changed
- Replaced the rounded-square "G" placeholder on the login page with the gaggle goose logo image (`/gaggle-goose.png`) inside the same rounded square container.
- File: `web/src/pages/login-lab/variants/StepFlow.tsx:41-45` — the `LoginPage` renders `StepFlow`, so login now shows the goose logo instead of the "G" text.

## Why
- Login page used a text "G" lettermark; request was to show the gaggle goose inside that rounded-square badge.

## Files touched
- `web/src/pages/login-lab/variants/StepFlow.tsx` — swapped `G` span contents for `<img src="/gaggle-goose.png" ... className="h-full w-full object-cover" />`, added `overflow-hidden` to the wrapper and dropped the text color/font classes.

## Preview
- Frontend: http://localhost:5215
- API: http://localhost:2115

## Reviewer notes
- Verify the 80×80 `gaggle-goose.png` (8-bit sRGB) reads well at 56×56 (h-14/w-14, rounded-2xl). White goose on the primary-filled square; if the bird needs more padding, add `p-1` or `object-contain` instead of `object-cover`.
- Other login-lab variants (CenteredBrand, Glassmorphism, SplitPanel, SplitStepFlow) still show "G" — untouched since only `StepFlow` is the live login; change them too if needed.
- Lint/build verified via the preview Docker build (vite build passed).
