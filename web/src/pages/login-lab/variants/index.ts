import type { ComponentType } from 'react';
import { SplitPanel } from './SplitPanel';
import { Glassmorphism } from './Glassmorphism';
import { CenteredBrand } from './CenteredBrand';
import { Minimal } from './Minimal';
import { StepFlow } from './StepFlow';
import { SplitStepFlow } from './SplitStepFlow';

export interface LoginVariant {
  id: string;
  name: string;
  description: string;
  Category: string;
  Component: ComponentType;
}

export const loginVariants: LoginVariant[] = [
  {
    id: 'split-panel',
    name: 'Split panel',
    description: 'Brand panel on one side, form on the other.',
    Category: 'Style',
    Component: SplitPanel,
  },
  {
    id: 'split-step-flow',
    name: 'Split step flow',
    description: 'Split panel layout with one-field-at-a-time steps.',
    Category: 'Style',
    Component: SplitStepFlow,
  },
  {
    id: 'glassmorphism',
    name: 'Glassmorphism',
    description: 'Frosted card floating over an animated gradient.',
    Category: 'Style',
    Component: Glassmorphism,
  },
  {
    id: 'centered-brand',
    name: 'Centered brand',
    description: 'Big wordmark with a compact centered form.',
    Category: 'Style',
    Component: CenteredBrand,
  },
  {
    id: 'minimal',
    name: 'Minimal',
    description: 'Quiet, text-only, whitespace-forward.',
    Category: 'Style',
    Component: Minimal,
  },
  {
    id: 'step-flow',
    name: 'Step flow',
    description: 'One field at a time: identifier, then password.',
    Category: 'Flow',
    Component: StepFlow,
  },
];