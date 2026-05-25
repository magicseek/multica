# Fix Request Efficient Help Floating Layout

## Problem

The Request efficient info affordance in the agent inspector currently renders
its explanatory copy inline under the toggle row. Opening the help changes the
property grid height and disrupts the inspector layout.

## Requirements

- The Request efficient property row must keep the same compact inspector row
  layout as the surrounding runtime/model/protocol rows.
- The help copy must float above the page content and not participate in the
  property grid layout.
- The help must remain reachable by click and discoverable by hover or focus.
- The help copy must still explain request-efficient mode in product language.
- Component tests must cover the floating disclosure behavior so future changes
  do not accidentally render the help inline again.

## Non-goals

- Redesigning the agent inspector or changing other agent settings behavior.
- Changing the semantics of request-efficient mode.
