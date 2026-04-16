---
title: About public access requests
weight: 20
---

Drynn's default stance is that accounts only exist because an administrator
decided they should. New users join by redeeming an invitation code that an
admin deliberately sent to a specific email address. There is no public
sign-up page, and that is not an accident: it keeps the user base small,
accountable, and easy to reason about.

Public access requests are the escape valve on that stance. They give
prospective users a way to raise their hand without giving them a way to
create an account on their own. The form at `/request-access` collects an
email and an optional reason, emails them to the configured administrator
contact, and returns the visitor to a "thanks, we'll be in touch" page. The
admin then decides — out of band — whether to send a real invitation from
the admin console. Nothing in the database changes as a result of the
request; the only artefact is the email in the admin's inbox.

## Why it is opt-in

A public form that emails the admin is useful right up until the moment it
becomes a vector for spam, harvesting, or social-engineering probes. For a
small template targeting a single droplet, that moment can arrive quickly.
So the feature ships off by default, behind two switches that both have to
be set before it activates:

- `request_access_enabled` (env: `REQUEST_ACCESS_ENABLED`) — the operator's
  explicit "yes, I want this".
- `admin_contact_email` (env: `ADMIN_CONTACT_EMAIL`) — the address the
  request is routed to.

The server refuses to expose the form unless *both* are set. If the flag is
on but no contact email is configured, the route returns 404 as though the
feature didn't exist. That pairing is deliberate: an enabled form with
nowhere to send mail is worse than a disabled form, because the visitor
thinks they've been heard when they haven't.

## How it relates to invitations

Access requests are a prelude to invitations, not a replacement for them.
When an admin decides a request is legitimate, the follow-up is the same
flow that already existed: open the admin console, create an invitation for
that email, let the invitation service send the code. The request form
exists so a stranger can ask; the invitation system exists so the admin can
grant. Keeping those two things separate is what lets the admin stay in
the loop without having to advertise their personal address on Discord or
in a README.

## The layered defences

Three small things keep the form honest:

1. **A honeypot field.** The form contains a visually hidden input that a
   human will never fill in. Naive form-scraping bots fill every input they
   see; when the server notices a non-empty honeypot it silently pretends
   the submission succeeded and discards it. No email is sent, and the bot
   gets no signal that it was caught.
2. **IP-based rate limiting.** The POST endpoint shares the same
   token-bucket limiter as `/signin`, `/forgot-password`, and
   `/reset-password`. A single source cannot drown the admin's inbox even
   if it gets past the honeypot.
3. **Silent failure on the send path.** If Mailgun is down or
   misconfigured, the visitor still sees the success page and the error is
   logged for the operator. This keeps the form from leaking operational
   state to the outside world and avoids giving an attacker a way to
   distinguish "configured" from "unconfigured" deployments.

None of these make the form spam-proof. They make it inconvenient enough
that the spam that does arrive is manageable by a human skimming their
inbox.

## When to turn it on

Enable public access requests when you have a clear, slow trickle of
strangers who want in, an admin who is willing to triage their inbox, and
a real `admin_contact_email` that the admin actually monitors. Leave it off
when any of those are missing. The feature is designed to fail closed, and
a closed form is a perfectly acceptable steady state for a template that
starts its life with three friends and a shared password manager.
