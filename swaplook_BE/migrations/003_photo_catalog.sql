-- Replace illustrated fixtures with openly licensed Wikimedia Commons photos (see docs/CATALOG-CREDITS.md).
-- SKU IDs, variants and stock are unchanged so existing decisions and merchant edits stay valid.
UPDATE products AS p SET name=v.name, price=v.price, color=v.color, material=v.material, cut=v.cut, image=v.image
FROM (VALUES
 ('SL-001','Oxford Blue Shirt',4800,'blue','cotton','regular','/catalog/SL-001.jpg'),
 ('SL-002','Harbor Stripe Popover',4200,'blue','cotton','regular','/catalog/SL-002.jpg'),
 ('SL-003','Graphite Check Shirt',4600,'grey','cotton','regular','/catalog/SL-003.jpg'),
 ('SL-004','Midnight Flannel Shirt',5400,'blue','flannel','regular','/catalog/SL-004.jpg'),
 ('SL-005','Picnic Check Camp Shirt',3900,'red','cotton','relaxed','/catalog/SL-005.jpg'),
 ('SL-006','Bubble Logo Tee',3200,'white','cotton','relaxed','/catalog/SL-006.jpg'),
 ('SL-007','Cloud Outline Tee',3000,'white','cotton','relaxed','/catalog/SL-007.jpg'),
 ('SL-008','Number One Tee',2800,'white','cotton','relaxed','/catalog/SL-008.jpg'),
 ('SL-009','Pitch Raglan Polo',3600,'black','polyester','regular','/catalog/SL-009.jpg'),
 ('SL-010','Sailor Middy Top',4400,'white','cotton','relaxed','/catalog/SL-010.jpg'),
 ('SL-011','Matchday Stripe Jersey',4000,'navy','polyester','regular','/catalog/SL-011.jpg'),
 ('SL-012','Ivory French Cuff Shirt',6400,'ivory','cotton','regular','/catalog/SL-012.jpg')
) AS v(id,name,price,color,material,cut,image)
WHERE p.id=v.id;
INSERT INTO schema_migrations(version) VALUES(3);
