-- Sample data for the dialect-corpus schema (applied from HCL, form_fields.options = json)
BEGIN;

-- ── Users (application.users) ────────────────────────────────────────────────
INSERT INTO application.users (id, first_name, last_name, email, phone, external_id, created_at) VALUES
('11111111-1111-1111-1111-000000000001', 'Amara',  'Okafor',   'amara.okafor@verdantgrid.io',    '+234 802 111 0001', 'cog|usr_amara',  now() - interval '400 days'),
('11111111-1111-1111-1111-000000000002', 'Daniel', 'Reyes',    'daniel.reyes@sunfieldcap.com',   '+1 415 555 0102',   'cog|usr_daniel', now() - interval '380 days'),
('11111111-1111-1111-1111-000000000003', 'Mei',    'Tanaka',   'mei.tanaka@asiacleanlab.jp',     '+81 3 5555 0103',   'cog|usr_mei',    now() - interval '350 days'),
('11111111-1111-1111-1111-000000000004', 'Lars',   'Bergström','lars.bergstrom@nordicgrid.se',   '+46 8 555 0104',    'cog|usr_lars',   now() - interval '320 days'),
('11111111-1111-1111-1111-000000000005', 'Priya',  'Nair',     'priya.nair@climatelens.in',      '+91 22 5550 0105',  'cog|usr_priya',  now() - interval '300 days'),
('11111111-1111-1111-1111-000000000006', 'Sofia',  'Marchetti','sofia.marchetti@terracircle.it', '+39 02 5550 0106',  'cog|usr_sofia',  now() - interval '260 days'),
('11111111-1111-1111-1111-000000000007', 'Jamal',  'Whitfield','jamal.whitfield@cityofdenver.gov','+1 303 555 0107',  'cog|usr_jamal',  now() - interval '200 days'),
('11111111-1111-1111-1111-000000000008', 'Nadia',  'Haddad',   'nadia.haddad@oasisventures.ae',  '+971 4 555 0108',   'cog|usr_nadia',  now() - interval '150 days');

-- ── Organization + membership ────────────────────────────────────────────────
INSERT INTO application.organization (id, name, slug, owner_id, metadata, created_at) VALUES
('22222222-2222-2222-2222-000000000001', 'Verdant Grid Collective', 'verdant-grid',
 '11111111-1111-1111-1111-000000000001',
 '{"address":"14 Marina Way, Lagos","phone_number":"+234 802 111 0000","email":"hello@verdantgrid.io","logo_id":"file_logo_vgc"}'::jsonb,
 now() - interval '390 days');

INSERT INTO application.organization_membership (organization_id, user_id) VALUES
('22222222-2222-2222-2222-000000000001', '11111111-1111-1111-1111-000000000001'),
('22222222-2222-2222-2222-000000000001', '11111111-1111-1111-1111-000000000005');

-- ── Week banners ─────────────────────────────────────────────────────────────
INSERT INTO application.week_banners (id, slug, name, description, theme_color, start_date, end_date, status, host_user_id, about, city, country, timezone, website_url, contact_email, published_at, approved_by_user_id, approved_at, approval_authority, created_at) VALUES
('33333333-3333-3333-3333-000000000001', 'climate-week-nairobi-2026', 'Climate Week Nairobi 2026',
 'A week of summits, expos and field trips accelerating East Africa''s green transition.',
 '#1f7a4d', DATE '2026-10-05', DATE '2026-10-09', 'live', '11111111-1111-1111-1111-000000000001',
 '<p>Five days of programming across Nairobi: policy roundtables, an investor expo, community energy site visits and a youth climate assembly.</p>',
 'Nairobi', 'Kenya', 'Africa/Nairobi', 'https://climateweeknairobi.example.org', 'hello@climateweeknairobi.example.org',
 now() - interval '45 days', '11111111-1111-1111-1111-000000000007', now() - interval '44 days', 'superadmin',
 now() - interval '120 days'),
('33333333-3333-3333-3333-000000000002', 'clean-capital-asia-2026', 'Clean Capital Asia 2026',
 'Where Asian climate startups meet institutional capital.',
 '#0e5a8a', DATE '2026-11-16', DATE '2026-11-18', 'pending_approval', '11111111-1111-1111-1111-000000000003',
 NULL, 'Singapore', 'Singapore', 'Asia/Singapore', 'https://cleancapitalasia.example.org', 'team@cleancapitalasia.example.org',
 NULL, NULL, NULL, NULL,
 now() - interval '20 days');

-- ── Events ───────────────────────────────────────────────────────────────────
INSERT INTO application.events
(id, slug, week_id, title, summary, description, format, participation_type, capacity, theme_color,
 cover_image_url, location, venue_address, city, country, latitude, longitude, sector,
 target_audience, outcomes, organiser_name, organiser_website_url, event_tags,
 status, host_user_id, start_time, end_time, timezone, published_at, approved_by_user_id, approved_at,
 approval_authority, registration_opens_at, registration_closes_at, price_amount, price_currency,
 contact_email, virtual_url, waitlist_open, created_at)
VALUES
('44444444-4444-4444-4444-000000000001', 'grid-scale-storage-summit', '33333333-3333-3333-3333-000000000001',
 'Grid-Scale Storage Summit', 'Financing and deploying utility-scale batteries across East Africa.',
 'A full-day summit for utilities, IPPs and storage developers: revenue stacking, tariff design, and the region''s first 1 GWh procurement pipeline.',
 'in_person', 'application', 250, '#1f7a4d',
 'https://cdn.example.org/covers/grid-storage.jpg', 'KICC, Conference Hall A', ' Harambee Ave, Nairobi', 'Nairobi', 'Kenya',
 -1.2864, 36.8172, 'energy_storage', 'Utilities, developers, development finance institutions',
 'A signed statement of interest from at least two DFIs to co-finance the pilot pipeline.',
 'Verdant Grid Collective', 'https://verdantgrid.io',
 '["Networking","Policy","Energy Storage"]',
 'live', '11111111-1111-1111-1111-000000000001',
 TIMESTAMPTZ '2026-10-06 09:00:00+03', TIMESTAMPTZ '2026-10-06 17:30:00+03', 'Africa/Nairobi',
 now() - interval '45 days', '11111111-1111-1111-1111-000000000007', now() - interval '44 days', 'superadmin',
 now() - interval '60 days', TIMESTAMPTZ '2026-10-05 23:59:00+03', NULL, 'USD',
 'storage@verdantgrid.io', NULL, true, now() - interval '110 days'),

('44444444-4444-4444-4444-000000000002', 'investor-speed-matchmaking', '33333333-3333-3333-3333-000000000001',
 'Investor Speed Matchmaking', 'Fifteen-minute curated 1:1s between climate founders and investors.',
 'Curated matchmaking sessions. Startups are matched with investors by sector, stage and ticket size using their profiles.',
 'in_person', 'application', 120, '#8a4f0e',
 'https://cdn.example.org/covers/matchmaking.jpg', 'Serena Hotel, Ballroom 2', 'Kenyatta Ave, Nairobi', 'Nairobi', 'Kenya',
 -1.2864, 36.8172, 'climate_finance', 'Seed to Series B founders and active climate investors',
 'At least 40 scheduled 1:1 meetings with follow-up commitments.',
 'Verdant Grid Collective', 'https://verdantgrid.io',
 '["Fundraising","Matchmaking","Investors"]',
 'live', '11111111-1111-1111-1111-000000000002',
 TIMESTAMPTZ '2026-10-07 10:00:00+03', TIMESTAMPTZ '2026-10-07 16:00:00+03', 'Africa/Nairobi',
 now() - interval '43 days', '11111111-1111-1111-1111-000000000007', now() - interval '42 days', 'superadmin',
 now() - interval '60 days', TIMESTAMPTZ '2026-10-06 23:59:00+03', NULL, 'USD',
 'matchmaking@verdantgrid.io', NULL, true, now() - interval '108 days'),

('44444444-4444-4444-4444-000000000003', 'youth-climate-assembly', '33333333-3333-3333-3333-000000000001',
 'Youth Climate Assembly', 'Open town hall: youth priorities for COP31 and beyond.',
 'A free public assembly with panels, workshops and an open-mic plenary. No application needed.',
 'hybrid', 'open', 500, '#0e8a5f',
 NULL, 'UoN Chancellor''s Court', 'University Way, Nairobi', 'Nairobi', 'Kenya',
 -1.2797, 36.8160, 'climate_policy', 'Students, youth organizers, first-time attendees',
 'A youth priorities memorandum delivered to the week''s closing plenary.',
 'Verdant Grid Collective', 'https://verdantgrid.io',
 '["Community","Policy","Youth"]',
 'live', '11111111-1111-1111-1111-000000000001',
 TIMESTAMPTZ '2026-10-08 14:00:00+03', TIMESTAMPTZ '2026-10-08 18:00:00+03', 'Africa/Nairobi',
 now() - interval '40 days', '11111111-1111-1111-1111-000000000001', now() - interval '39 days', 'weekhost',
 now() - interval '60 days', TIMESTAMPTZ '2026-10-08 12:00:00+03', NULL, 'USD',
 'youth@verdantgrid.io', 'https://meet.example.org/youth-assembly', true, now() - interval '100 days'),

('44444444-4444-4444-4444-000000000004', 'regen-ag-field-trip', '33333333-3333-3333-3333-000000000001',
 'Regenerative Agriculture Field Trip', 'Site visit to two regenerative farms in Machakos County.',
 'A guided day trip to smallholder regenerative farms demonstrating agroforestry and biochar systems. Travel and lunch included.',
 'in_person', 'paid', 40, '#5a6e0e',
 NULL, 'Machakos County (bus departs KICC)', 'Assembly at KICC main gate', 'Machakos', 'Kenya',
 -1.5177, 37.2634, 'agriculture', 'Investors and corporates exploring nature-based solutions',
 'Direct relationships with two farmer cooperatives for offtake pilots.',
 'Verdant Grid Collective', 'https://verdantgrid.io',
 '["Site Visit","Agriculture","Nature"]',
 'live', '11111111-1111-1111-1111-000000000005',
 TIMESTAMPTZ '2026-10-09 07:30:00+03', TIMESTAMPTZ '2026-10-09 18:00:00+03', 'Africa/Nairobi',
 now() - interval '38 days', '11111111-1111-1111-1111-000000000001', now() - interval '37 days', 'weekhost',
 now() - interval '55 days', TIMESTAMPTZ '2026-10-07 23:59:00+03', 120.00, 'USD',
 'fieldtrips@verdantgrid.io', NULL, false, now() - interval '95 days'),

('44444444-4444-4444-4444-000000000005', 'clean-capital-pitch-day', '33333333-3333-3333-3333-000000000002',
 'Clean Capital Pitch Day', 'Twelve startups pitch to a panel of Asia-focused climate funds.',
 'Invite-only pitch day concluding Clean Capital Asia. Twelve companies, seven minutes each, live Q&A with the investment committee.',
 'in_person', 'invite_only', 80, '#0e5a8a',
 NULL, 'Marina Bay Sands, Level 3', '10 Bayfront Ave', 'Singapore', 'Singapore',
 1.2834, 103.8607, 'climate_finance', 'Pre-selected startups and invited investors only',
 'Two or more term sheets initiated from the room.',
 'Asia Clean Lab', 'https://asiacleanlab.jp',
 '["Fundraising","Pitch","Investors"]',
 'draft', '11111111-1111-1111-1111-000000000003',
 TIMESTAMPTZ '2026-11-18 09:00:00+08', TIMESTAMPTZ '2026-11-18 15:00:00+08', 'Asia/Singapore',
 NULL, NULL, NULL, NULL,
 now() - interval '15 days', TIMESTAMPTZ '2026-11-17 23:59:00+08', NULL, 'USD',
 'pitches@asiacleanlab.jp', NULL, true, now() - interval '18 days'),

('44444444-4444-4444-4444-000000000006', 'green-hydrogen-webinar', NULL,
 'Green Hydrogen: Hype vs Bankability', 'A candid webinar on which hydrogen projects actually reach FID.',
 'Virtual panel with lenders and project developers dissecting three case studies: one financed, one stalled, one abandoned.',
 'virtual', 'open', 1000, '#4a0e8a',
 NULL, NULL, NULL, NULL, NULL,
 NULL, NULL, 'hydrogen', 'Project developers, lenders, policy analysts',
 'A public scoring rubric for bankable hydrogen projects.',
 'Climate Lens', 'https://climatelens.in',
 '["Webinar","Hydrogen","Finance"]',
 'completed', '11111111-1111-1111-1111-000000000005',
 TIMESTAMPTZ '2026-06-11 15:00:00+05:30', TIMESTAMPTZ '2026-06-11 16:30:00+05:30', 'Asia/Kolkata',
 now() - interval '150 days', '11111111-1111-1111-1111-000000000007', now() - interval '148 days', 'superadmin',
 now() - interval '160 days', TIMESTAMPTZ '2026-06-11 14:00:00+05:30', NULL, 'USD',
 'webinars@climatelens.in', 'https://meet.example.org/h2-bankability', true, now() - interval '170 days');

-- ── Event co-hosts ───────────────────────────────────────────────────────────
INSERT INTO application.event_cohosts (event_id, user_id, permissions, granted_by_user_id, accepted_at) VALUES
('44444444-4444-4444-4444-000000000001', '11111111-1111-1111-1111-000000000005',
 '{"analytics":"view","page":"edit","guests":"edit","comms":"edit"}'::json,
 '11111111-1111-1111-1111-000000000001', now() - interval '100 days'),
('44444444-4444-4444-4444-000000000002', '11111111-1111-1111-1111-000000000008',
 '{"analytics":"view","page":"edit","guests":"none","comms":"none"}'::json,
 '11111111-1111-1111-1111-000000000002', now() - interval '90 days');

-- ── Form fields (options is a real JSON array now) ───────────────────────────
INSERT INTO application.form_fields
(id, event_id, label, field_type, placeholder, options, required, is_core, help_text, is_locked, position)
VALUES
-- Event 1: Grid-Scale Storage Summit
('55555555-5555-5555-5555-000000000001', '44444444-4444-4444-4444-000000000001', 'Full name', 'short_text', 'Jane Wanjiru', '[]'::json, true, true, NULL, true, 0),
('55555555-5555-5555-5555-000000000002', '44444444-4444-4444-4444-000000000001', 'Work email', 'email', 'you@company.com', '[]'::json, true, true, NULL, true, 1),
('55555555-5555-5555-5555-000000000003', '44444444-4444-4444-4444-000000000001', 'Organization', 'short_text', 'Acme Power Ltd', '[]'::json, true, true, NULL, true, 2),
('55555555-5555-5555-5555-000000000004', '44444444-4444-4444-4444-000000000001', 'Stakeholder type', 'single_choice', 'Select one',
 '["Utility / Transmission Operator","Independent Power Producer","Battery / Storage Developer","Development Finance Institution","Commercial Bank","Private Equity / Venture Capital","Government / Regulator","Project Developer (Solar / Wind)","EPC & Engineering","Equipment Manufacturer","Grid Services / Aggregator","Research & Academia","Consultancy","Other"]'::json,
 true, true, 'Used to route you to the right roundtable track.', true, 3),
('55555555-5555-5555-5555-000000000005', '44444444-4444-4444-4444-000000000001', 'Topics you want covered', 'multi_choice', 'Select all that apply',
 '["Revenue stacking & merchant risk","Tariff and PPA design for storage","Ancillary services market rules","Grid interconnection queues","Second-life batteries","Long-duration storage technologies","Safeguards & ESG compliance","Local content requirements","Cybersecurity for grid assets"]'::json,
 false, false, NULL, false, 4),
('55555555-5555-5555-5555-000000000006', '44444444-4444-4444-4444-000000000001', 'Pipeline stage of your storage projects', 'single_choice', 'Select one',
 '["Pre-feasibility","Feasibility / permitting","Financial close","Under construction","Operating","No projects yet - exploring"]'::json,
 false, false, 'Helps us calibrate the finance deep-dive.', false, 5),
('55555555-5555-5555-5555-000000000007', '44444444-4444-4444-4444-000000000001', 'Dietary requirements', 'multi_choice', 'Select all that apply',
 '["No restrictions","Vegetarian","Vegan","Halal","Kosher","Gluten-free","Lactose-free","Nut allergy","Shellfish allergy"]'::json,
 false, false, NULL, false, 6),
('55555555-5555-5555-5555-000000000008', '44444444-4444-4444-4444-000000000001', 'Country', 'country', 'Kenya', '[]'::json, false, false, NULL, false, 7),

-- Event 2: Investor Speed Matchmaking
('55555555-5555-5555-5555-000000000101', '44444444-4444-4444-4444-000000000002', 'Full name', 'short_text', NULL, '[]'::json, true, true, NULL, true, 0),
('55555555-5555-5555-5555-000000000102', '44444444-4444-4444-4444-000000000002', 'Work email', 'email', NULL, '[]'::json, true, true, NULL, true, 1),
('55555555-5555-5555-5555-000000000103', '44444444-4444-4444-4444-000000000002', 'Organization', 'short_text', NULL, '[]'::json, true, true, NULL, true, 2),
('55555555-5555-5555-5555-000000000104', '44444444-4444-4444-4444-000000000002', 'I am attending as', 'single_choice', NULL,
 '["Startup seeking capital","Investor (VC / PE)","Family office","Impact fund","Development finance institution","Corporate strategic investor","Angel syndicate","Crowdfunding platform","Blended finance facility"]'::json,
 true, true, 'Matchmaking only works with both sides present.', true, 3),
('55555555-5555-5555-5555-000000000105', '44444444-4444-4444-4444-000000000002', 'Sectors of interest', 'multi_choice', NULL,
 '["Solar & distributed energy","Wind power","Energy storage","Green hydrogen","E-mobility & EV charging","Regenerative agriculture","Forestry & land use","Circular economy & waste","Industrial heat & cement","Water & sanitation","Climate fintech","Carbon markets","Adaptation & resilience","Blue economy","Built environment"]'::json,
 true, false, 'Pick up to five for the best matches.', false, 4),
('55555555-5555-5555-5555-000000000106', '44444444-4444-4444-4444-000000000002', 'Funding stage (startups)', 'single_choice', NULL,
 '["Idea / pre-seed","Pre-seed","Seed","Seed extension","Series A","Series B","Growth","Not fundraising"]'::json,
 false, false, 'Investors filter on this field.', false, 5),
('55555555-5555-5555-5555-000000000107', '44444444-4444-4444-4444-000000000002', 'Ticket size range (investors)', 'single_choice', NULL,
 '["< $100k","$100k - $500k","$500k - $2M","$2M - $5M","$5M - $20M","$20M+"]'::json,
 false, false, NULL, false, 6),

-- Event 3: Youth Climate Assembly
('55555555-5555-5555-5555-000000000201', '44444444-4444-4444-4444-000000000003', 'Full name', 'short_text', NULL, '[]'::json, true, true, NULL, true, 0),
('55555555-5555-5555-5555-000000000202', '44444444-4444-4444-4444-000000000003', 'Email', 'email', NULL, '[]'::json, true, true, NULL, true, 1),
('55555555-5555-5555-5555-000000000203', '44444444-4444-4444-4444-000000000003', 'How would you like to join?', 'single_choice', NULL,
 '["In person in Nairobi","Virtually (Zoom)"]'::json,
 false, false, NULL, false, 2),
('55555555-5555-5555-5555-000000000204', '44444444-4444-4444-4444-000000000003', 'Afternoon workshops you want', 'multi_choice', NULL,
 '["Storytelling for climate action","Community energy cooperatives 101","Policy advocacy bootcamp","Careers in the green economy","Air quality citizen science","Climate finance for beginners","Zero-waste event design"]'::json,
 false, false, 'Seats are first come, first served.', false, 3),
('55555555-5555-5555-5555-000000000205', '44444444-4444-4444-4444-000000000003', 'Languages you speak', 'multi_choice', NULL,
 '["English","Kiswahili","French","Arabic","Amharic","Luo","Kikuyu","Somali","Other"]'::json,
 false, false, NULL, false, 4);

-- ── Registrations (responses keyed by form_field id) ────────────────────────
INSERT INTO application.registrations
(id, event_id, user_id, name, first_name, last_name, email, organization, stakeholder_type, status,
 responses, qr_hash, checked_in, check_in_time, source, phone, created_at)
VALUES
('66666666-6666-6666-6666-000000000001', '44444444-4444-4444-4444-000000000001', '11111111-1111-1111-1111-000000000003',
 'Mei Tanaka', 'Mei', 'Tanaka', 'mei.tanaka@asiacleanlab.jp', 'Asia Clean Lab', 'Equipment Manufacturer', 'approved',
 '{"55555555-5555-5555-5555-000000000003":"Asia Clean Lab","55555555-5555-5555-5555-000000000004":"Equipment Manufacturer","55555555-5555-5555-5555-000000000005":["Long-duration storage technologies","Local content requirements"],"55555555-5555-5555-5555-000000000006":"Operating","55555555-5555-5555-5555-000000000007":["No restrictions"]}'::json,
 'qr_hash_mei_storage', true, TIMESTAMPTZ '2026-10-06 08:42:00+03', 'self', '+81 3 5555 0103', now() - interval '50 days'),

('66666666-6666-6666-6666-000000000002', '44444444-4444-4444-4444-000000000001', '11111111-1111-1111-1111-000000000004',
 'Lars Bergström', 'Lars', 'Bergström', 'lars.bergstrom@nordicgrid.se', 'Nordic Grid AB', 'Utility / Transmission Operator', 'approved',
 '{"55555555-5555-5555-5555-000000000003":"Nordic Grid AB","55555555-5555-5555-5555-000000000004":"Utility / Transmission Operator","55555555-5555-5555-5555-000000000005":["Ancillary services market rules","Grid interconnection queues","Cybersecurity for grid assets"],"55555555-5555-5555-5555-000000000006":"Operating","55555555-5555-5555-5555-000000000007":["Lactose-free"]}'::json,
 'qr_hash_lars_storage', false, NULL, 'self', '+46 8 555 0104', now() - interval '48 days'),

('66666666-6666-6666-6666-000000000003', '44444444-4444-4444-4444-000000000001', NULL,
 'Grace Wanjiru', 'Grace', 'Wanjiru', 'grace.wanjiru@kenpower.co.ke', 'KenPower Utilities', 'Utility / Transmission Operator', 'pending',
 '{"55555555-5555-5555-5555-000000000003":"KenPower Utilities","55555555-5555-5555-5555-000000000004":"Utility / Transmission Operator","55555555-5555-5555-5555-000000000005":["Tariff and PPA design for storage","Revenue stacking & merchant risk"],"55555555-5555-5555-5555-000000000006":"Feasibility / permitting"}'::json,
 NULL, false, NULL, 'self', '+254 733 555 011', now() - interval '12 days'),

('66666666-6666-6666-6666-000000000004', '44444444-4444-4444-4444-000000000002', '11111111-1111-1111-1111-000000000006',
 'Sofia Marchetti', 'Sofia', 'Marchetti', 'sofia.marchetti@terracircle.it', 'Terra Circle Capital', 'Investor (VC / PE)', 'approved',
 '{"55555555-5555-5555-5555-000000000104":"Investor (VC / PE)","55555555-5555-5555-5555-000000000105":["Energy storage","Circular economy & waste","Climate fintech"],"55555555-5555-5555-5555-000000000107":"$2M - $5M"}'::json,
 'qr_hash_sofia_match', false, NULL, 'self', '+39 02 5550 0106', now() - interval '30 days'),

('66666666-6666-6666-6666-000000000005', '44444444-4444-4444-4444-000000000002', '11111111-1111-1111-1111-000000000008',
 'Nadia Haddad', 'Nadia', 'Haddad', 'nadia.haddad@oasisventures.ae', 'Oasis Ventures', 'Startup seeking capital', 'approved',
 '{"55555555-5555-5555-5555-000000000104":"Startup seeking capital","55555555-5555-5555-5555-000000000105":["Solar & distributed energy","Water & sanitation"],"55555555-5555-5555-5555-000000000106":"Seed"}'::json,
 NULL, false, NULL, 'invite', '+971 4 555 0108', now() - interval '25 days'),

('66666666-6666-6666-6666-000000000006', '44444444-4444-4444-4444-000000000003', NULL,
 'Brian Otieno', 'Brian', 'Otieno', 'brian.otieno@student.uon.ac.ke', 'University of Nairobi', 'student', 'approved',
 '{"55555555-5555-5555-5555-000000000203":"In person in Nairobi","55555555-5555-5555-5555-000000000204":["Storytelling for climate action","Careers in the green economy"],"55555555-5555-5555-5555-000000000205":["English","Kiswahili","Luo"]}'::json,
 'qr_hash_brian_youth', false, NULL, 'self', NULL, now() - interval '8 days'),

('66666666-6666-6666-6666-000000000007', '44444444-4444-4444-4444-000000000003', NULL,
 'Amina Hassan', 'Amina', 'Hassan', 'amina.hassan@youthnet.or.ke', 'YouthNet Kenya', 'ngo', 'pending',
 '{"55555555-5555-5555-5555-000000000203":"Virtually (Zoom)","55555555-5555-5555-5555-000000000204":["Policy advocacy bootcamp"],"55555555-5555-5555-5555-000000000205":["English","Kiswahili","Somali"]}'::json,
 NULL, false, NULL, 'self', NULL, now() - interval '5 days'),

('66666666-6666-6666-6666-000000000008', '44444444-4444-4444-4444-000000000004', '11111111-1111-1111-1111-000000000002',
 'Daniel Reyes', 'Daniel', 'Reyes', 'daniel.reyes@sunfieldcap.com', 'Sunfield Capital', 'Investor', 'pending',
 '{}'::json, NULL, false, NULL, 'self', '+1 415 555 0102', now() - interval '3 days');

-- ── Meetings ────────────────────────────────────────────────────────────────
INSERT INTO application.meetings (id, event_id, requester_user_id, recipient_user_id, slot_start, slot_end, status, message, channels) VALUES
('99999999-9999-9999-9999-000000000001', '44444444-4444-4444-4444-000000000002',
 '11111111-1111-1111-1111-000000000008', '11111111-1111-1111-1111-000000000006',
 TIMESTAMPTZ '2026-10-07 10:00:00+03', TIMESTAMPTZ '2026-10-07 10:15:00+03', 'accepted',
 'Oasis Ventures: distributed solar + water for Gulf SMEs. Raising a $3M seed.', '["in_app","email"]'::jsonb),
('99999999-9999-9999-9999-000000000002', '44444444-4444-4444-4444-000000000002',
 '11111111-1111-1111-1111-000000000002', '11111111-1111-1111-1111-000000000004',
 TIMESTAMPTZ '2026-10-07 11:00:00+03', TIMESTAMPTZ '2026-10-07 11:15:00+03', 'slot_proposed',
 NULL, '["in_app"]'::jsonb);

-- ── Taxonomy terms ──────────────────────────────────────────────────────────
INSERT INTO application.taxonomy_terms (id, kind, slug, label, parent_id, ordinal, position) VALUES
('77777777-7777-7777-7777-000000000001', 'stakeholder_type', 'startup-founder',        'Startup Founder',        NULL, NULL, 0),
('77777777-7777-7777-7777-000000000002', 'stakeholder_type', 'investor',               'Investor',               NULL, NULL, 1),
('77777777-7777-7777-7777-000000000003', 'stakeholder_type', 'policy-maker',           'Policy Maker',           NULL, NULL, 2),
('77777777-7777-7777-7777-000000000004', 'stakeholder_type', 'researcher',             'Researcher',             NULL, NULL, 3),
('77777777-7777-7777-7777-000000000005', 'stakeholder_type', 'civil-society',          'NGO / Civil Society',    NULL, NULL, 4),
('77777777-7777-7777-7777-000000000006', 'stakeholder_type', 'corporate-sustainability','Corporate Sustainability Lead', NULL, NULL, 5),
('77777777-7777-7777-7777-000000000007', 'industry_focus',   'renewable-energy',       'Renewable Energy',       NULL, NULL, 10),
('77777777-7777-7777-7777-000000000008', 'industry_focus',   'energy-storage',         'Energy Storage',         '77777777-7777-7777-7777-000000000007', NULL, 11),
('77777777-7777-7777-7777-000000000009', 'industry_focus',   'agriculture',            'Agriculture',            NULL, NULL, 12),
('77777777-7777-7777-7777-00000000000a', 'industry_focus',   'climate-finance',        'Climate Finance',        NULL, NULL, 13),
('77777777-7777-7777-7777-00000000000b', 'supply_chain_stage','upstream',              'Upstream',               NULL, 1, 20),
('77777777-7777-7777-7777-00000000000c', 'supply_chain_stage','midstream',             'Midstream',              NULL, 2, 21),
('77777777-7777-7777-7777-00000000000d', 'supply_chain_stage','downstream',            'Downstream',             NULL, 3, 22),
('77777777-7777-7777-7777-00000000000e', 'startup_stage',    'pre-seed',               'Pre-Seed',               NULL, 1, 30),
('77777777-7777-7777-7777-00000000000f', 'startup_stage',    'seed',                   'Seed',                   NULL, 2, 31),
('77777777-7777-7777-7777-000000000010', 'startup_stage',    'series-a',               'Series A',               NULL, 3, 32),
('77777777-7777-7777-7777-000000000011', 'event_tag',        'networking',             'Networking',             NULL, NULL, 40),
('77777777-7777-7777-7777-000000000012', 'event_tag',        'fundraising',            'Fundraising',            NULL, NULL, 41),
('77777777-7777-7777-7777-000000000013', 'target_audience',  'first-time-founders',    'First-time Founders',    NULL, NULL, 50);

INSERT INTO application.event_taxonomy_terms (event_id, term_id) VALUES
('44444444-4444-4444-4444-000000000001', '77777777-7777-7777-7777-000000000008'),
('44444444-4444-4444-4444-000000000001', '77777777-7777-7777-7777-000000000011'),
('44444444-4444-4444-4444-000000000002', '77777777-7777-7777-7777-000000000012'),
('44444444-4444-4444-4444-000000000002', '77777777-7777-7777-7777-000000000011');

-- ── Profiles ────────────────────────────────────────────────────────────────
INSERT INTO application.profiles
(id, kind, slug, user_id, display_name, headline, bio, job_title, organisation_name, city, country,
 founded_year, team_size, fundraising_active, funding_ask_amount, currency, socials, attributes, is_published)
VALUES
('88888888-8888-8888-8888-000000000001', 'individual', 'mei-tanaka', '11111111-1111-1111-1111-000000000003',
 'Mei Tanaka', 'Battery systems engineer turning utility pilots into bankable projects.',
 'Fifteen years across cell manufacturing and grid integration in Japan and Kenya.',
 'CTO', 'Asia Clean Lab', 'Tokyo', 'Japan', NULL, NULL, false, NULL, 'USD',
 '{"linkedin":"https://linkedin.com/in/mei-tanaka"}'::jsonb,
 '{"speaking_topics":["second-life batteries","grid services"],"open_to_meetings":true}'::jsonb, true),
('88888888-8888-8888-8888-000000000002', 'startup', 'oasis-ventures', '11111111-1111-1111-1111-000000000008',
 'Oasis Ventures', 'Solar-powered water ATMs for off-grid communities in the Gulf.',
 'Deployed 220 water ATMs serving 90,000 people; expanding to Oman and Saudi Arabia.',
 NULL, NULL, 'Dubai', 'United Arab Emirates', 2022, 18, true, 3000000.00, 'USD',
 '{"linkedin":"https://linkedin.com/company/oasis-ventures","x":"https://x.com/oasisventures"}'::jsonb,
 '{"unit_economics":"LTV/CAC 4.1x","certifications":["ISO 9001"],"revenue_model":"pay-per-liter subscription"}'::jsonb, true),
('88888888-8888-8888-8888-000000000003', 'investor', 'terra-circle-capital', NULL,
 'Terra Circle Capital', 'Early-growth fund for circular economy and climate fintech in EMEA.',
 '$120M Fund I. 14 investments. Cheques $2-5M with follow-on reserves.',
 NULL, NULL, 'Milan', 'Italy', 2021, 12, false, NULL, 'EUR',
 '{"linkedin":"https://linkedin.com/company/terra-circle"}'::jsonb,
 '{"cheque_min":2000000,"cheque_max":5000000,"sectors":["circular economy","climate fintech","energy storage"]}'::jsonb, true),
('88888888-8888-8888-8888-000000000004', 'corporate', 'kenpower-utilities', NULL,
 'KenPower Utilities', 'Regional utility piloting East Africa''s first 1 GWh storage tender.',
 NULL, NULL, NULL, 'Nairobi', 'Kenya', 1954, 2400, false, NULL, 'USD',
 '{}'::jsonb, '{"procurement_pipeline":"1 GWh BESS tender opening Q1 2027"}'::jsonb, true);

INSERT INTO application.profile_members (profile_id, user_id, role_label, is_owner) VALUES
('88888888-8888-8888-8888-000000000002', '11111111-1111-1111-1111-000000000008', 'Founder & CEO', true),
('88888888-8888-8888-8888-000000000003', '11111111-1111-1111-1111-000000000006', 'Partner', true);

INSERT INTO application.profile_taxonomy_terms (profile_id, term_id, is_focus) VALUES
('88888888-8888-8888-8888-000000000002', '77777777-7777-7777-7777-000000000007', false),
('88888888-8888-8888-8888-000000000002', '77777777-7777-7777-7777-00000000000e', false),
('88888888-8888-8888-8888-000000000003', '77777777-7777-7777-7777-000000000008', true),
('88888888-8888-8888-8888-000000000003', '77777777-7777-7777-7777-00000000000a', true);

INSERT INTO application.profile_geographies (profile_id, scope, country, city) VALUES
('88888888-8888-8888-8888-000000000002', 'operating', 'AE', 'Dubai'),
('88888888-8888-8888-8888-000000000002', 'operating', 'OM', 'Muscat'),
('88888888-8888-8888-8888-000000000003', 'investment_focus', 'KE', 'Nairobi'),
('88888888-8888-8888-8888-000000000003', 'investment_focus', 'IT', 'Milan');

-- ── Business cards & checklists ─────────────────────────────────────────────
INSERT INTO application.business_cards (owner_user_id, scanned_user_id, event_id, display_name, title, organization, email, capture_source) VALUES
('11111111-1111-1111-1111-000000000002', '11111111-1111-1111-1111-000000000003', '44444444-4444-4444-4444-000000000001',
 'Mei Tanaka', 'CTO', 'Asia Clean Lab', 'mei.tanaka@asiacleanlab.jp', 'qr'),
('11111111-1111-1111-1111-000000000002', '11111111-1111-1111-1111-000000000004', '44444444-4444-4444-4444-000000000001',
 'Lars Bergström', 'Head of Grid Development', 'Nordic Grid AB', 'lars.bergstrom@nordicgrid.se', 'qr');

INSERT INTO application.user_business_cards (user_id, share_token, display_name, title, organisation, email, phone, visible_fields) VALUES
('11111111-1111-1111-1111-000000000003', 'tok_mei_9f2a', 'Mei Tanaka', 'CTO', 'Asia Clean Lab', 'mei.tanaka@asiacleanlab.jp', '+81 3 5555 0103',
 '{"phone":false}'::jsonb),
('11111111-1111-1111-1111-000000000002', 'tok_dan_4c8b', 'Daniel Reyes', 'Principal', 'Sunfield Capital', 'daniel.reyes@sunfieldcap.com', '+1 415 555 0102',
 '{}'::jsonb);

INSERT INTO application.event_checklist_items (event_id, item_key, label, completed, auto, position) VALUES
('44444444-4444-4444-4444-000000000001', 'create_event', 'Create event', true, true, 0),
('44444444-4444-4444-4444-000000000001', 'build_form', 'Build registration form', true, true, 1),
('44444444-4444-4444-4444-000000000001', 'submit_for_approval', 'Submit for approval', true, true, 2),
('44444444-4444-4444-4444-000000000001', 'invite_speakers', 'Invite speakers', true, false, 3),
('44444444-4444-4444-4444-000000000001', 'send_reminder', 'Send pre-event reminder', false, false, 4),
('44444444-4444-4444-4444-000000000001', 'check_in_desk', 'Set up check-in desk', false, false, 5);

-- ── Communications ──────────────────────────────────────────────────────────
INSERT INTO application.communications
(id, event_id, type, name, subject, preview_text, body, segment, recipient_count, status, sent_by, sent_at, channel, created_at)
VALUES
('aaaaaaaa-aaaa-aaaa-aaaa-000000000001', '44444444-4444-4444-4444-000000000001', 'reminder', 'T-7 reminder',
 'One week to go: Grid-Scale Storage Summit', 'Your agenda, dietary notes and venue map inside.',
 '<p>Hi {{name}}, the summit is next Tuesday. Doors open 08:30 at KICC Hall A.</p>',
 'approved', 2, 'sent', '11111111-1111-1111-1111-000000000001', now() - interval '7 days', 'email', now() - interval '8 days'),
('aaaaaaaa-aaaa-aaaa-aaaa-000000000002', '44444444-4444-4444-4444-000000000001', 'nudge', 'Pending approvals nudge',
 '3 registrations are waiting for your review', 'Review pending applications to keep your guest list moving.',
 '<p>You have pending registrations waiting for approval.</p>', 'all', 1, 'scheduled', NULL, NULL, 'email', now() - interval '1 day');

-- ── Consents ────────────────────────────────────────────────────────────────
INSERT INTO application.consents (consent_type, user_id, registration_id, event_id, granted, text_version, source, granted_at) VALUES
('attendee_showcase', '11111111-1111-1111-1111-000000000003', NULL, '44444444-4444-4444-4444-000000000001', true, 'attendee_showcase.v2', 'registration_form', now() - interval '50 days'),
('matchmaking',      '11111111-1111-1111-1111-000000000006', NULL, '44444444-4444-4444-4444-000000000002', true, 'matchmaking.v1', 'registration_form', now() - interval '30 days'),
('photography',      NULL, '66666666-6666-6666-6666-000000000006', '44444444-4444-4444-4444-000000000003', true, 'photography.v1', 'registration_form', now() - interval '8 days'),
('marketing_email',  '11111111-1111-1111-1111-000000000004', NULL, NULL, false, 'marketing_email.v3', 'profile_settings', NULL);

COMMIT;
