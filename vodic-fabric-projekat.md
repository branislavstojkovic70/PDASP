# Vodič za izradu projekta - Hyperledger Fabric trgovački sistem

## Faza 0 - Priprema (pre pisanja koda)

1. Instalirati Docker, Docker Compose, Go (1.19+), Node.js (za SDK i CLI alate), i binarne fajlove Hyperledger Fabric-a (verzija >= 2.2.6) preko `fabric-samples` install skripte.
2. Proći kroz `fabric-samples/test-network` primer da se razume osnovni tok (bringup mreže, kreiranje kanala, deploy chaincode-a) pre nego što pravite sopstvenu mrežu od nule.
3. Napraviti privatan/školski GitHub repo i odmah dodati `NebojsaHorvat` kao saradnika - ovo je eksplicitan zahtev iz specifikacije i lako se zaboravi.
4. Podeliti posao u parovima tako da se to jasno vidi kroz commit-e (npr. jedan radi mrežu + chaincode, drugi SDK aplikaciju + skripte, pa obrnuto za drugi deo, ili podela po funkcionalnostima).

## Faza 1 - Dizajn mreže (na papiru pre nego u kodu)

Pre pisanja `docker-compose` fajlova, skicirajte:

- **3 organizacije** (npr. Org1, Org2, Org3), svaka sa po **3 peer-a** (peer0, peer1, peer2).
- **2 kanala** (npr. `channel1`, `channel2`) - svi peer-ovi su na oba kanala.
- **Orderer organizacija** sa RAFT konsenzusom (preporuka: 3 orderer čvora zbog RAFT-a, mada specifikacija traži samo "bar 1 orderer po kanalu").
- Svaki peer je i endorser i committer (ovo je podrazumevano ponašanje Fabric peer-a, samo obratite pažnju na endorsement policy da ne isključi nijednu organizaciju ako to ne želite).
- Block cut parametri: `BatchTimeout: 1s`, `MaxMessageCount: 2` - ovo direktno pokriva zahtev "2 transakcije u bloku ili svaka sekunda".

Imenovanje (predlog): `Org1MSP/Org2MSP/Org3MSP`, `OrdererMSP`, `channel1/channel2`, `peer0.org1.example.com` itd.

## Faza 2 - Generisanje kripto materijala i mrežnih fajlova

1. Napisati `crypto-config.yaml` (za `cryptogen`) ili konfiguraciju za Fabric CA - **CA je obavezan**, znači koristite Fabric CA server za svaku organizaciju, ne samo `cryptogen`.
2. Napisati `configtx.yaml` sa definicijama profila za genesis blok i za oba kanala.
3. Napisati `docker-compose.yaml` (ili više fajlova) koji podiže:
   - 3x CA kontejner (po jedan za svaku org, + eventualno za orderer org)
   - 9 peer kontejnera (3 org x 3 peer-a)
   - CouchDB kontejner **za svaki peer** (state database)
   - orderer kontejner(e)
   - CLI kontejner za pomoćne komande
4. Napisati `.sh` skripte:
   - `generate.sh` - generiše sve kripto materijale i genesis blok
   - `network-up.sh` / `network-down.sh` - diže/gasi docker mrežu
   - `create-channels.sh` - kreira oba kanala i dodaje sve peer-ove
5. Testirati da mreža uspešno startuje i da se kanali kreiraju pre nego što pređete na chaincode.

## Faza 3 - Chaincode (preporuka: Go)

### 3.1 Definisati strukture (JSON tagovi obavezni jer se sve čuva kao JSON u world state-u)

```go
type Trgovac struct {
    IdTrgovca string    `json:"idTrgovca"`
    Tip       string    `json:"tip"`
    PIB       string    `json:"pib"`
    Proizvodi []string  `json:"proizvodi"` // šifre proizvoda
    Racuni    []string  `json:"racuni"`    // id-jevi računa
    Stanje    float64   `json:"stanje"`
}

type Proizvod struct {
    Sifra      string  `json:"sifra"`
    Ime        string  `json:"ime"`
    RokTrajanja string `json:"rokTrajanja,omitempty"`
    Cena       float64 `json:"cena"`
    Kolicina   int     `json:"kolicina"`
    TrgovacId  string  `json:"trgovacId"` // korisno za pretragu po trgovcu
}

type Korisnik struct {
    IdKorisnika string   `json:"idKorisnika"`
    Ime         string   `json:"ime"`
    Prezime     string   `json:"prezime"`
    Email       string   `json:"email"`
    Racuni      []string `json:"racuni"`
    Stanje      float64  `json:"stanje"`
}

type Racun struct {
    Id        string `json:"id"`
    TrgovacId string `json:"trgovacId"`
    KorisnikId string `json:"korisnikId"`
    ProizvodSifra string `json:"proizvodSifra"`
    Datum     string `json:"datum"`
}
```

Napomena: kod korišćenja `PutState`, ključevi u world state-u treba da imaju prefiks po tipu (npr. `TRGOVAC_id1`, `PROIZVOD_sifra1`, `KORISNIK_id1`, `RACUN_id1`) da se izbegnu kolizije i da composite key pretrage rade lakše.

### 3.2 Funkcije chaincode-a (minimalni spisak)

- `InitLedger()` - upisuje minimum 2 trgovca (svaki sa >= 2 proizvoda) i nekoliko korisnika
- `UnosTrgovca(id, tip, pib)`
- `DodajProizvod(trgovacId, sifra, ime, rok, cena, kolicina)` - podržati i dodavanje više proizvoda odjednom (npr. JSON niz kao parametar)
- `UnosKorisnika(id, ime, prezime, email)`
- `KupiProizvod(korisnikId, trgovacId, proizvodSifra)`:
  - proveriti da korisnik postoji, proizvod postoji, ima dovoljno količine
  - proveriti `korisnik.Stanje >= proizvod.Cena`, u suprotnom vratiti grešku
  - umanjiti količinu (ako padne na 0, obrisati proizvod iz world state-a i iz liste trgovca)
  - kreirati novi `Racun`, dodati mu ID i u `Korisnik.Racuni` i u `Trgovac.Racuni`
  - umanjiti stanje korisnika, uvećati stanje trgovca
- `UplataNaRacun(tipEntiteta, id, iznos)` - uplata i za korisnika i za trgovca
- Query funkcije (koristiti CouchDB rich queries sa selector-ima):
  - `PretraziProizvodePoImenu(ime)`
  - `PretraziProizvodePoSifri(sifra)`
  - `PretraziProizvodePoTipuTrgovca(tip)`
  - `PretraziProizvodePoCeni(min, max)`
  - `PretraziProizvodeKombinovano(map parametara)` - dinamički sastavljen CouchDB selector

### 3.3 Obrada grešaka

Svaka funkcija mora vratiti jasnu grešku (`fmt.Errorf`) za slučajeve: entitet ne postoji, nedovoljno sredstava, nedovoljna količina, nevalidni parametri. Nemojte panic-ovati - chaincode treba da vraća grešku koju SDK aplikacija hvata i lepo prikazuje.

### 3.4 CouchDB rich query primer

```go
queryString := fmt.Sprintf(`{"selector":{"docType":"proizvod","cena":{"$gte":%f,"$lte":%f}}}`, min, max)
resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
```

Za odbranu pripremite objašnjenje: LevelDB podržava samo upit po ključu ili opsegu ključeva (range query po composite key-jevima), dok CouchDB omogućava upit po sadržaju JSON polja (npr. cena, tip, ime) bez potrebe da to bude deo ključa - to je suštinska razlika koju treba demonstrirati sa bar 2-3 "bogata" upita (npr. kombinacija cene i tipa trgovca, ili pretraga po opsegu roka trajanja).

## Faza 4 - Deploy chaincode-a

1. Napisati `deploy-chaincode.sh` skriptu koja pakuje, instalira na sve peer-ove, odobrava (approve) za sve 3 organizacije, i commit-uje chaincode na oba kanala.
2. Definisati endorsement policy (npr. "AND(Org1MSP.peer, Org2MSP.peer, Org3MSP.peer)" ili "OutOf" varijanta - vaš izbor, samo obrazložite na odbrani).
3. Pozvati `InitLedger` invoke da se postavi početno stanje.

## Faza 5 - SDK konzolna aplikacija (Node.js preporuka - fabric-network / fabric-gateway SDK)

### 5.1 Enroll / Login

- Koristiti Fabric CA client SDK da se registruje i enroll-uje admin za svaku organizaciju, zatim da se registruju obični korisnici (npr. `user1@org1`, `user1@org2`).
- Sertifikate/wallet čuvati lokalno (npr. filesystem wallet).
- Aplikacija treba da omogući biranje sa kojim identitetom se konektuje (bar 2 sertifikata iz različitih organizacija).

### 5.2 Query i Invoke

- Koristiti `Gateway.connect()` sa izabranim identitetom, dobiti `Network` -> `Contract`, pa pozivati `evaluateTransaction` (query) i `submitTransaction` (invoke).
- Sve rezultate parsirati iz Buffer-a u JSON i vraćati JSON korisniku (kako specifikacija traži).

### 5.3 Predloženi CLI meni (ili komandno-linijski argumenti)

```
1) Enroll/Login (izbor organizacije i korisnika)
2) Unos trgovca
3) Dodavanje proizvoda
4) Unos korisnika
5) Kupovina proizvoda
6) Uplata na račun
7) Pretraga proizvoda (po imenu/šifri/tipu/ceni/kombinovano)
8) Izlaz
```

Implementirati try/catch oko svakog SDK poziva i mapirati chaincode greške u čitljive poruke.

## Faza 6 - Testiranje

1. Napisati `.sh` skripte koje pozivaju vašu Node aplikaciju (ili direktno `peer chaincode invoke/query` za brzu proveru) za svaku funkcionalnost:
   - `test-enroll.sh`, `test-unos-trgovca.sh`, `test-kupovina.sh`, `test-query-kombinovano.sh` itd.
2. Testirati i granične/error slučajeve: kupovina bez dovoljno sredstava, kupovina nepostojećeg proizvoda, upit za nepostojećeg korisnika.
3. Testirati da transakcije rade sa oba sertifikata (dve organizacije) i na oba kanala.

## Faza 7 - Dokumentacija i priprema za odbranu

- README sa uputstvom za pokretanje mreže, deploy chaincode-a i pokretanje aplikacije.
- Kratko objašnjenje arhitekture (dijagram organizacija/kanala je poželjan).
- Pripremiti objašnjenje CouchDB vs LevelDB upita (traženo eksplicitno u specifikaciji).
- Proveriti da GitHub istorija jasno pokazuje ravnomeran doprinos oba člana tima.

## Predlog redosleda rada (praktični workflow)

1. Mreža (Faza 1-2) dok radi jedan diže test-network kao referencu
2. Chaincode strukture i osnovne funkcije (Faza 3) - testirati direktno preko `peer chaincode` CLI komandi, bez SDK-a, radi brzine
3. Deploy skripte (Faza 4)
4. SDK aplikacija (Faza 5) - paralelno sa chaincode radom, koristeći test-network kao privremenu mrežu dok sopstvena mreža nije gotova
5. Spajanje sopstvene mreže sa SDK aplikacijom i pisanje test skripti (Faza 6)
6. Dokumentacija (Faza 7)

Ovaj redosled omogućava da dvoje ljudi rade paralelno - jedno na mreži/chaincode-u, drugo na SDK aplikaciji protiv `test-network`-a, pa se kasnije spoji.
