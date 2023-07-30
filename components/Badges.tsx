import Link from 'next/link'
import Image from 'next/image'

export default function Badges({
  apple = "https://podcasts.apple.com/kr/podcast/retrotech-%ED%8C%9F%EC%BA%90%EC%8A%A4%ED%8A%B8/id1698903712",
  youtube = "https://www.youtube.com/playlist?list=PLEHf_UYxvkp9HCnP3UIZhEss_Yo4XuUDX",
  google = "https://podcasts.google.com/feed/aHR0cHM6Ly9yZXRyb3RlY2gub3V0c2lkZXIuZGV2L2ZlZWQueG1s",
  spotify = "https://open.spotify.com/show/3nSplj43Rd86snTrsEHdTI"
}) {
    return (
        <div className="badges">
            <Link href={apple}>
                <Image src="/badges/apple.svg" alt="Listen on Apple Podcasts" width={300} height={0} priority className="badge" />
            </Link>
            <Link href={youtube} className="youtube">
                <Image src="/badges/youtube.svg" alt="Available on YouTube" width={240} height={0} priority className="badge youtube" />
            </Link>
            <Link href={google}>
                <Image src="/badges/google.svg" alt="Listen on Google Podcasts" width={300} height={0} priority className="badge" />
            </Link>
            <Link href={spotify}>
                <Image src="/badges/spotify.svg" alt="Listen on Spotify" width={300} height={0} priority className="badge spotify" />
            </Link>
        </div>
    )
}
