-- The Stylebook canvas logical space grew from 400x500 to 400x711, so saved
-- items are shifted down by (711 - 500) / 2 = 105.5 to stay visually centred.
UPDATE LookItems SET y = y + 105.5;
