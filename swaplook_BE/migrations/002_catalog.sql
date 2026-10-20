INSERT INTO products(id,name,price,currency,color,material,cut,category,image) VALUES
 ('SL-001','The Everyday Shirt',4800,'USD','cream','cotton','relaxed','tops','/catalog/SL-001.svg'),
 ('SL-002','Sunday Cotton Shirt',4200,'USD','cream','cotton','relaxed','tops','/catalog/SL-002.svg'),
 ('SL-003','The Linen Edit',5400,'USD','cream','linen','relaxed','tops','/catalog/SL-003.svg'),
 ('SL-004','Soft Structure Shirt',4600,'USD','sage','cotton','relaxed','tops','/catalog/SL-004.svg'),
 ('SL-005','Coastline Stripe',4400,'USD','blue','cotton','relaxed','tops','/catalog/SL-005.svg'),
 ('SL-006','Daybreak Poplin',3900,'USD','white','cotton','regular','tops','/catalog/SL-006.svg'),
 ('SL-007','Terracotta Weekend',4900,'USD','terracotta','linen','relaxed','tops','/catalog/SL-007.svg'),
 ('SL-008','The Midnight Shirt',5200,'USD','black','cotton','regular','tops','/catalog/SL-008.svg'),
 ('SL-009','Petal Cotton Blouse',4500,'USD','pink','cotton','relaxed','tops','/catalog/SL-009.svg'),
 ('SL-010','Olive Studio Top',5800,'USD','sage','linen','regular','tops','/catalog/SL-010.svg'),
 ('SL-011','Cloud Knit Tee',3600,'USD','cream','knit','regular','tops','/catalog/SL-011.svg'),
 ('SL-012','Blue Hour Shirt',6200,'USD','blue','linen','relaxed','tops','/catalog/SL-012.svg');
INSERT INTO variants(product_id,size,quantity)
 SELECT p.id,s.size,CASE WHEN p.id='SL-001' AND s.size IN ('M','L') THEN 0
 WHEN p.id='SL-003' AND s.size='XL' THEN 0 ELSE 2+(ascii(right(p.id,1))%5) END
 FROM products p CROSS JOIN (VALUES('XS'),('S'),('M'),('L'),('XL')) AS s(size);
INSERT INTO schema_migrations(version) VALUES(2);
