// Optional PostgreSQL-WASM smoke check. This does not verify pgx/network/row-lock concurrency.
// Install @electric-sql/pglite in tmp/pg-tools before running this script.
import { PGlite } from '../tmp/pg-tools/node_modules/@electric-sql/pglite/dist/index.js'
import { readFile } from 'node:fs/promises'
import assert from 'node:assert/strict'

const db = new PGlite()
try {
  for (const file of ['001_initial.sql', '002_catalog.sql']) await db.exec(await readFile(new URL(`../swaplook_BE/migrations/${file}`, import.meta.url), 'utf8'))
  const catalog = await db.query(`SELECT p.id,p.name,p.price,p.currency,p.color,p.material,p.cut,p.category,p.image,jsonb_object_agg(v.size,v.quantity) AS stock FROM products p JOIN variants v ON v.product_id=p.id GROUP BY p.id ORDER BY p.id`)
  assert.equal(catalog.rows.length, 12)
  assert.equal(catalog.rows[0].stock.M, 0)
  assert.equal((await db.query('SELECT count(*) AS count FROM variants')).rows[0].count, 60)
  await assert.rejects(() => db.query("UPDATE variants SET quantity=-1 WHERE product_id='SL-002' AND size='M'"))
  await assert.rejects(() => db.query("INSERT INTO variants VALUES ('missing','M',1)"))
  await db.query("INSERT INTO sessions(id,state,expires_at) VALUES($1,$2,now()+interval '1 day')", ['fixture-session', JSON.stringify({ shortlist: null, decision: null, demo: true })])
  await db.query('INSERT INTO render_jobs(id,session_id,state,task_id) VALUES($1,$2,$3,$4)', ['fixture-job', 'fixture-session', JSON.stringify({ status: 'pending', image: '' }), 'private-task-fixture'])
  await db.exec(`UPDATE render_jobs SET state=state || '{"status":"deleted","image":"","message":"Application image copies deleted."}'::jsonb,task_id='' WHERE session_id='fixture-session'`)
  assert.equal((await db.query('SELECT state FROM render_jobs')).rows[0].state.status, 'deleted')
  await db.transaction(async tx => {
    const row = await tx.query(`SELECT p.id,p.price,v.quantity FROM products p JOIN variants v ON v.product_id=p.id WHERE p.id=$1 AND v.size=$2 FOR UPDATE OF p,v`, ['SL-002', 'M'])
    assert.ok(row.rows[0].quantity > 0)
  })
  await db.exec("DELETE FROM sessions WHERE id='fixture-session'")
  assert.equal((await db.query('SELECT count(*) AS count FROM render_jobs')).rows[0].count, 0)
  console.log('PASS: two migrations, 12 products/60 variants, catalog query, stock/FK constraints, row-lock query syntax, JSON job deletion and cascading media deletion in PostgreSQL WASM.')
  console.log('LIMIT: pgx wire protocol and multi-connection PostgreSQL transaction behavior were not exercised.')
} finally { await db.close() }
